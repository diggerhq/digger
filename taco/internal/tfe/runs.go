package tfe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)


func (h *TfeHandler) GetRun(c echo.Context) error {
	ctx := c.Request().Context()
	runID := c.Param("id")

	// Get run from database
	run, err := h.runRepo.GetRun(ctx, runID)
	if err != nil {
		fmt.Printf("Failed to get run %s: %v\n", runID, err)
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Run %s not found", runID),
			}},
		})
	}

	// Determine if run is confirmable (waiting for user approval)
	isConfirmable := run.Status == "planned_and_finished" && run.CanApply && !run.AutoApply
	
	fmt.Printf("[GetRun] 🔍 Poll: runID=%s, status=%s, canApply=%v, autoApply=%v, planOnly=%v, isConfirmable=%v\n", 
		run.ID, run.Status, run.CanApply, run.AutoApply, run.PlanOnly, isConfirmable)

	// Use unit ID as workspace ID (they're the same in our architecture)
	workspaceID := run.UnitID
	
	// Build response
	response := tfe.TFERun{
		ID:        run.ID,
		Status:    run.Status,
		IsDestroy: run.IsDestroy,
		Message:   run.Message,
		PlanOnly:  run.PlanOnly,
		Actions: &tfe.RunActions{
			IsCancelable:  run.IsCancelable,
			IsConfirmable: isConfirmable,
			CanApply:      run.CanApply,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: workspaceID,
		},
		ConfigurationVersion: &tfe.ConfigurationVersionRef{
			ID: run.ConfigurationVersionID,
		},
	}

	if run.PlanID != nil {
		response.Plan = &tfe.PlanRef{ID: *run.PlanID}
	}

	// Include apply reference when run is applying or applied
	// In our simplified model, apply ID is the same as run ID
	if run.Status == "applying" || run.Status == "applied" || run.Status == "apply_queued" {
		response.Apply = &tfe.ApplyRef{ID: run.ID}
		fmt.Printf("[GetRun] Added Apply reference: applyID=%s for status=%s\n", run.ID, run.Status)
	} else {
		fmt.Printf("[GetRun] No Apply reference (status=%s)\n", run.Status)
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		fmt.Printf("error marshaling run payload: %v\n", err)
		return err
	}
	return nil
}


func (h *TfeHandler) CreateRun(c echo.Context) error {
	ctx := c.Request().Context()

	// Decode the JSON:API request
	var requestData struct {
		Data struct {
			Attributes struct {
				Message   string `json:"message"`
				IsDestroy bool   `json:"is-destroy"`
				AutoApply bool   `json:"auto-apply"` // Terraform CLI sends this with -auto-approve
			} `json:"attributes"`
			Relationships struct {
				Workspace struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"workspace"`
				ConfigurationVersion struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"configuration-version"`
			} `json:"relationships"`
		} `json:"data"`
	}

	// Manually decode JSON since content-type is application/vnd.api+json
	if err := json.NewDecoder(c.Request().Body).Decode(&requestData); err != nil {
		fmt.Printf("Failed to decode request: %v\n", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "400",
				"title":  "bad request",
				"detail": "Invalid request format",
			}},
		})
	}

	workspaceID := requestData.Data.Relationships.Workspace.Data.ID
	cvID := requestData.Data.Relationships.ConfigurationVersion.Data.ID
	message := requestData.Data.Attributes.Message
	isDestroy := requestData.Data.Attributes.IsDestroy
	autoApply := requestData.Data.Attributes.AutoApply
	
	// Log the full request for debugging
	fmt.Printf("📥 CreateRun request: message=%q, isDestroy=%v, autoApply=%v, workspaceID=%s\n", 
		message, isDestroy, autoApply, workspaceID)

	// Get org and user context from middleware
	orgIdentifier, _ := c.Get("organization_id").(string)
	userID, _ := c.Get("user_id").(string)
	
	if orgIdentifier == "" {
		orgIdentifier = "default-org" // Fallback for testing
	}
	if userID == "" {
		userID = "system"
	}

	// Resolve external org ID (e.g., "org_01K9X3...") to internal UUID
	// This is critical for S3 path construction: <orgUUID>/<unitUUID>/terraform.tfstate
	orgUUID, err := h.identifierResolver.ResolveOrganization(ctx, orgIdentifier)
	if err != nil {
		fmt.Printf("Failed to resolve organization %s: %v\n", orgIdentifier, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": fmt.Sprintf("Failed to resolve organization: %v", err),
			}},
		})
	}
	fmt.Printf("Resolved org identifier '%s' to UUID '%s'\n", orgIdentifier, orgUUID)

	// Strip ws- prefix from workspace ID to get the actual unit ID
	unitID := convertWorkspaceToStateID(workspaceID)

	// Create the run in database
	run := &domain.TFERun{
		OrgID:                  orgUUID, // Store UUID, not external ID!
		UnitID:                 unitID,
		Status:                 "pending",
		IsDestroy:              isDestroy,
		Message:                message,
		PlanOnly:               false, // Always false for apply operations (terraform apply with or without -auto-approve)
		AutoApply:              autoApply, // Only auto-trigger if -auto-approve was used
		Source:                 "cli",
		IsCancelable:           true,
		CanApply:               false,
		ConfigurationVersionID: cvID,
		CreatedBy:              userID,
	}
	
	fmt.Printf("Creating run: autoApply=%v, planOnly=%v\n", autoApply, run.PlanOnly)

	if err := h.runRepo.CreateRun(ctx, run); err != nil {
		fmt.Printf("Failed to create run: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to create run",
			}},
		})
	}

	fmt.Printf("Created run %s for unit %s\n", run.ID, unitID)

	// Create a plan for this run
	plan := &domain.TFEPlan{
		OrgID:     orgUUID,
		RunID:     run.ID,
		Status:    "pending",
		CreatedBy: userID,
	}

	if err := h.planRepo.CreatePlan(ctx, plan); err != nil {
		fmt.Printf("Failed to create plan: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to create plan",
			}},
		})
	}

	fmt.Printf("Created plan %s for run %s\n", plan.ID, run.ID)

	// Update run with plan ID
	if err := h.runRepo.UpdateRunPlanID(ctx, run.ID, plan.ID); err != nil {
		fmt.Printf("Failed to update run with plan ID: %v\n", err)
		// Non-fatal, continue
	}

	// Trigger real plan execution asynchronously
	go func() {
		fmt.Printf("[CreateRun] Starting async plan execution for run %s\n", run.ID)
		// Create plan executor
		executor := NewPlanExecutor(h.runRepo, h.planRepo, h.configVerRepo, h.blobStore)
		
		// Execute the plan (this will run terraform plan)
		if err := executor.ExecutePlan(context.Background(), run.ID); err != nil {
			fmt.Printf("[CreateRun] ❌ Plan execution failed for run %s: %v\n", run.ID, err)
		} else {
			fmt.Printf("[CreateRun] ✅ Plan execution completed successfully for run %s\n", run.ID)
		}
	}()

	// Return JSON:API response
	response := tfe.TFERun{
		ID:        run.ID,
		Status:    "planning", // Return as planning immediately
		IsDestroy: run.IsDestroy,
		Message:   run.Message,
		PlanOnly:  run.PlanOnly,
		Actions: &tfe.RunActions{
			IsCancelable: run.IsCancelable,
			CanApply:     run.CanApply,
		},
		Plan: &tfe.PlanRef{
			ID: plan.ID,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: workspaceID,
		},
		ConfigurationVersion: &tfe.ConfigurationVersionRef{
			ID: cvID,
		},
	}
	
	// For auto-apply runs, include Apply reference immediately so Terraform CLI knows to expect it
	if run.AutoApply {
		response.Apply = &tfe.ApplyRef{ID: run.ID}
		fmt.Printf("[CreateRun] Added Apply reference for auto-apply run: applyID=%s\n", run.ID)
	} else {
		fmt.Printf("[CreateRun] No Apply reference (AutoApply=false, user will confirm apply manually)\n")
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusCreated)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		fmt.Printf("error marshaling run payload: %v\n", err)
		return err
	}

	return nil
}

// Helper function for pointer to string
func stringPtr(s string) *string {
	return &s
}

// ApplyRun handles POST /runs/:id/actions/apply
func (h *TfeHandler) ApplyRun(c echo.Context) error {
	ctx := c.Request().Context()
	runID := c.Param("id")

	// Get run from database
	run, err := h.runRepo.GetRun(ctx, runID)
	if err != nil {
		fmt.Printf("Failed to get run %s: %v\n", runID, err)
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Run %s not found", runID),
			}},
		})
	}

	// Check if run can be applied
	if run.Status != "planned_and_finished" {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "409",
				"title":  "conflict",
				"detail": fmt.Sprintf("Run cannot be applied in status %s", run.Status),
			}},
		})
	}

	// Check if plan has changes
	if run.PlanID != nil {
		plan, err := h.planRepo.GetPlan(ctx, *run.PlanID)
		if err == nil && !plan.HasChanges {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"errors": []map[string]string{{
					"status": "409",
					"title":  "conflict",
					"detail": "Plan has no changes to apply",
				}},
			})
		}
	}

	fmt.Printf("Triggering apply for run %s\n", runID)

	// Update run status to apply_queued
	if err := h.runRepo.UpdateRunStatus(ctx, runID, "apply_queued"); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to queue apply",
			}},
		})
	}

	// Trigger real apply execution asynchronously
	go func() {
		// Create apply executor
		executor := NewApplyExecutor(h.runRepo, h.planRepo, h.configVerRepo, h.blobStore)
		
		// Execute the apply (this will run terraform apply)
		if err := executor.ExecuteApply(context.Background(), runID); err != nil {
			fmt.Printf("Apply execution failed for run %s: %v\n", runID, err)
		}
	}()

	// Return updated run
	run.Status = "apply_queued"
	response := tfe.TFERun{
		ID:        run.ID,
		Status:    run.Status,
		IsDestroy: run.IsDestroy,
		Message:   run.Message,
		PlanOnly:  run.PlanOnly,
		Actions: &tfe.RunActions{
			IsCancelable: false,
			CanApply:     false,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: run.UnitID,
		},
		ConfigurationVersion: &tfe.ConfigurationVersionRef{
			ID: run.ConfigurationVersionID,
		},
	}

	if run.PlanID != nil {
		response.Plan = &tfe.PlanRef{ID: *run.PlanID}
	}
	
	// Include Apply reference so Terraform CLI knows to fetch apply logs
	response.Apply = &tfe.ApplyRef{ID: run.ID}
	fmt.Printf("[ApplyRun] Added Apply reference: applyID=%s\n", run.ID)

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		fmt.Printf("error marshaling run payload: %v\n", err)
		return err
	}
	return nil
}


// GetRunEvents returns timeline events for a run (used by Terraform CLI to track progress)
func (h *TfeHandler) GetRunEvents(c echo.Context) error {
	ctx := c.Request().Context()
	runID := c.Param("id")

	// Get run to generate events based on status
	run, err := h.runRepo.GetRun(ctx, runID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Run %s not found", runID),
			}},
		})
	}

	// Generate events based on run status (JSON:API format with type field)
	events := []map[string]interface{}{}
	eventCounter := 0
	
	// Helper to create a properly formatted event
	addEvent := func(action, description string) {
		eventCounter++
		events = append(events, map[string]interface{}{
			"type": "run-events",
			"id":   fmt.Sprintf("%s-%d", runID, eventCounter),
			"attributes": map[string]interface{}{
				"action":      action,
				"created-at":  run.UpdatedAt.Format("2006-01-02T15:04:05Z"),
				"description": description,
			},
		})
	}
	
	// Always include "run created" event
	addEvent("created", "Run was created")

	// Add status-specific events
	switch run.Status {
	case "planning", "planned", "planned_and_finished":
		addEvent("planning", "Plan is running")
	case "applying", "applied":
		addEvent("planning", "Plan completed")
		addEvent("applying", "Apply is running")
	case "errored":
		addEvent("errored", "Run encountered an error")
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	return c.JSON(http.StatusOK, map[string]interface{}{"data": events})
}

// GetPolicyChecks returns Sentinel policy check results (enterprise feature - return empty)
func (h *TfeHandler) GetPolicyChecks(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
}

// GetTaskStages returns run task stages (newer feature - return empty)
func (h *TfeHandler) GetTaskStages(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
}

// GetCostEstimates returns cost estimation results (enterprise feature - return empty)
func (h *TfeHandler) GetCostEstimates(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
}

func (h *TfeHandler) EmptyListResponse(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	// We can't use jsonapi.MarshalPayload here because that produces
	// OnePayload or ManyPayload from structs. We just want a bare
	// `{ "data": [] }`, which is valid JSON:API for "no resources".
	_, err := c.Response().Writer.Write([]byte(`{"data":[]}`))
	if err != nil {
		return err
	}
	return nil
}