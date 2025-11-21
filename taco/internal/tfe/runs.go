package tfe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetRun(c echo.Context) error {
	ctx := c.Request().Context()
	runID := c.Param("id")

	logger := slog.Default().With(
		slog.String("operation", "get_run"),
		slog.String("run_id", runID),
	)

	// Get run from database
	run, err := h.runRepo.GetRun(ctx, runID)
	if err != nil {
		logger.Error("failed to get run", slog.String("error", err.Error()))
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Run %s not found", runID),
			}},
		})
	}

	includeParam := c.QueryParam("include")

	// Determine if run is confirmable (waiting for user approval)
	// Status is "planned" when waiting for confirmation
	isConfirmable := run.Status == "planned" && run.CanApply && !run.AutoApply

	// Determine if run has changes (Terraform CLI uses this!)
	var hasChanges bool
	var planData *domain.TFEPlan
	if run.PlanID != nil {
		needPlanData := run.Status == "planned" || contains(includeParam, "plan")
		if needPlanData {
			if plan, err := h.planRepo.GetPlan(ctx, *run.PlanID); err != nil {
				logger.Warn("failed to fetch plan for run", slog.String("error", err.Error()))
			} else {
				planData = plan
				if plan.Status == "finished" {
					hasChanges = plan.HasChanges
				}
			}
		}
	}

	logger.Info("GET /runs/:id - returning run state",
		slog.String("status", run.Status),
		slog.Bool("can_apply", run.CanApply),
		slog.Bool("auto_apply", run.AutoApply),
		slog.Bool("plan_only", run.PlanOnly),
		slog.Bool("is_confirmable", isConfirmable),
		slog.Bool("has_plan_id", run.PlanID != nil))

	// Use unit ID as workspace ID (they're the same in our architecture)
	// Terraform CLI expects workspace ID in the format "ws-{uuid}"
	workspaceID := "ws-" + run.UnitID

	// Build response
	response := tfe.TFERun{
		ID:            run.ID,
		Status:        run.Status,
		HasChanges:    hasChanges,
		IsDestroy:     run.IsDestroy,
		Message:       run.Message,
		PlanOnly:      run.PlanOnly,
		AutoApply:     run.AutoApply,
		IsConfirmable: isConfirmable,
		Actions: &tfe.RunActions{
			IsCancelable:  run.IsCancelable,
			IsConfirmable: isConfirmable,
		},
		Permissions: &tfe.RunPermissions{
			CanApply: run.CanApply,
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
		logger.Info("GET /runs/:id - added apply reference (run in progress/complete)", slog.String("apply_id", run.ID), slog.String("status", run.Status))
	} else {
		logger.Info("GET /runs/:id - NO apply reference (waiting for confirmation or still planning)",
			slog.String("status", run.Status),
			slog.Bool("is_confirmable", isConfirmable))
	}

	// Check if client requested to include plan details (Terraform CLI uses "?include=workspace,plan")
	// If yes, manually construct JSON:API response with plan in "included" array
	if run.PlanID != nil && contains(includeParam, "plan") {
		// Ensure we have plan data (may have been loaded already)
		if planData == nil {
			if plan, err := h.planRepo.GetPlan(ctx, *run.PlanID); err == nil {
				planData = plan
			} else {
				logger.Warn("failed to fetch plan for include=plan", slog.String("error", err.Error()))
			}
		}

		if planData != nil && planData.Status == "finished" {
			logger.Info("including full plan data for CLI",
				slog.Bool("has_changes", planData.HasChanges),
				slog.Int("adds", planData.ResourceAdditions))

			// Manually construct JSON:API compound document with included plan
			// First marshal the run using jsonapi to get proper structure
			var runBuffer bytes.Buffer
			if err := jsonapi.MarshalPayload(&runBuffer, &response); err != nil {
				logger.Error("error marshaling run", slog.String("error", err.Error()))
				return err
			}

			// Parse the marshaled run JSON
			var runDoc map[string]interface{}
			if err := json.Unmarshal(runBuffer.Bytes(), &runDoc); err != nil {
				logger.Error("error parsing run JSON", slog.String("error", err.Error()))
				return err
			}

			// Build plan data for included section
			publicBase := os.Getenv("OPENTACO_PUBLIC_BASE_URL")
			// Generate a fresh signed token for log streaming (path-based to survive CLI stripping queries)
			logToken, _ := auth.GenerateLogStreamToken(planData.ID, 24*time.Hour)
			logReadURL := fmt.Sprintf("%s/tfe/api/v2/plans/%s/logs/%s", publicBase, planData.ID, logToken)
			planData := map[string]interface{}{
				"id":   planData.ID,
				"type": "plans",
				"attributes": map[string]interface{}{
					"status":                planData.Status,
					"has-changes":           planData.HasChanges,
					"resource-additions":    planData.ResourceAdditions,
					"resource-changes":      planData.ResourceChanges,
					"resource-destructions": planData.ResourceDestructions,
					"log-read-url":          logReadURL,
				},
				"relationships": map[string]interface{}{
					"run": map[string]interface{}{
						"data": map[string]interface{}{
							"id":   run.ID,
							"type": "runs",
						},
					},
				},
			}

			// Add included section to the document
			runDoc["included"] = []interface{}{planData}

			c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
			c.Response().WriteHeader(http.StatusOK)
			if err := json.NewEncoder(c.Response().Writer).Encode(runDoc); err != nil {
				logger.Error("error encoding compound document", slog.String("error", err.Error()))
				return err
			}
			return nil
		}
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		logger.Error("error marshaling run payload", slog.String("error", err.Error()))
		return err
	}
	return nil
}

// Helper function to check if a comma-separated string contains a value
func contains(includeParam, value string) bool {
	if includeParam == "" {
		return false
	}
	for _, part := range splitCommaList(includeParam) {
		if part == value {
			return true
		}
	}
	return false
}

func splitCommaList(s string) []string {
	result := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func (h *TfeHandler) CreateRun(c echo.Context) error {
	ctx := c.Request().Context()

	logger := slog.Default().With(slog.String("operation", "create_run"))

	// Decode the JSON:API request
	var requestData struct {
		Data struct {
			Attributes struct {
				Message   string `json:"message"`
				IsDestroy bool   `json:"is-destroy"`
				AutoApply bool   `json:"auto-apply"` // Terraform CLI sends this with -auto-approve
				PlanOnly  *bool  `json:"plan-only"`  // Pointer to detect if CLI sent it (terraform plan vs apply)
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
		logger.Error("failed to decode request", slog.String("error", err.Error()))
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
	planOnlyFromCLI := requestData.Data.Attributes.PlanOnly // Pointer - can be nil

	// Log the full request for debugging
	planOnlyValue := "not-set"
	if planOnlyFromCLI != nil {
		if *planOnlyFromCLI {
			planOnlyValue = "true"
		} else {
			planOnlyValue = "false"
		}
	}
	logger.Info("create run request",
		slog.String("message", message),
		slog.Bool("is_destroy", isDestroy),
		slog.Bool("auto_apply_from_cli", autoApply),
		slog.String("plan_only_from_cli", planOnlyValue),
		slog.String("workspace_id", workspaceID))

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
		logger.Error("failed to resolve organization",
			slog.String("org_identifier", orgIdentifier),
			slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": fmt.Sprintf("Failed to resolve organization: %v", err),
			}},
		})
	}
	logger.Info("resolved organization",
		slog.String("org_identifier", orgIdentifier),
		slog.String("org_uuid", orgUUID))

	// Strip ws- prefix from workspace ID to get the actual unit ID
	unitID := convertWorkspaceToStateID(workspaceID)

	// Fetch unit to check workspace-level auto-apply setting
	unit, err := h.unitRepo.Get(ctx, unitID)
	if err != nil {
		logger.Error("failed to get unit for run",
			slog.String("unit_id", unitID),
			slog.String("error", err.Error()))
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Workspace %s not found", workspaceID),
			}},
		})
	}

	// Security: Remote runs require an active sandbox provider.
	executionMode := "local" // default
	if unit.TFEExecutionMode != nil && *unit.TFEExecutionMode != "" {
		executionMode = *unit.TFEExecutionMode
	}

	logger.Info("🔍 DECISION POINT: Checking execution mode",
		slog.String("unit_id", unitID),
		slog.String("unit_name", unit.Name),
		slog.String("execution_mode", executionMode),
		slog.Bool("sandbox_configured", h.sandbox != nil),
		slog.String("sandbox_provider", func() string {
			if h.sandbox != nil {
				return h.sandbox.Name()
			}
			return "none"
		}()))

	if executionMode == "remote" && h.sandbox == nil {
		logger.Warn("❌ BLOCKED: remote run creation - sandbox provider not configured",
			slog.String("unit_id", unitID),
			slog.String("execution_mode", executionMode))
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "403",
				"title":  "forbidden",
				"detail": "Remote execution mode requires configuring OPENTACO_SANDBOX_PROVIDER",
			}},
		})
	}

	if executionMode == "remote" {
		logger.Info("✅ APPROVED: Remote execution will be used",
			slog.String("unit_id", unitID),
			slog.String("sandbox_provider", h.sandbox.Name()))
	} else {
		logger.Info("ℹ️  Local execution mode - sandbox will not be used",
			slog.String("unit_id", unitID))
	}

	// Get the configuration version to check if it's speculative
	configVer, err := h.configVerRepo.GetConfigurationVersion(ctx, cvID)
	if err != nil {
		logger.Error("failed to get configuration version",
			slog.String("cv_id", cvID),
			slog.String("error", err.Error()))
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "400",
				"title":  "bad request",
				"detail": fmt.Sprintf("Configuration version %s not found", cvID),
			}},
		})
	}

	// Determine final auto-apply setting:
	// 1. If workspace has auto-apply enabled → auto-apply
	// 2. If CLI sends -auto-approve flag → auto-apply
	// 3. Either one being true results in auto-apply
	workspaceAutoApply := unit.TFEAutoApply != nil && *unit.TFEAutoApply
	finalAutoApply := workspaceAutoApply || autoApply

	logger.Info("determining auto-apply setting",
		slog.Bool("workspace_auto_apply", workspaceAutoApply),
		slog.Bool("cli_auto_approve", autoApply),
		slog.Bool("final_auto_apply", finalAutoApply))

	// Determine plan-only setting:
	// 1. If CLI explicitly sets it (terraform plan sends true) → use CLI value
	// 2. Otherwise inherit from configuration version's speculative flag
	planOnly := configVer.Speculative // Default: inherit from config version
	if planOnlyFromCLI != nil {
		planOnly = *planOnlyFromCLI // CLI explicitly set it - respect that
		logger.Info("CLI explicitly set plan-only", slog.Bool("plan_only", planOnly))
	}

	// Create the run in database
	run := &domain.TFERun{
		OrgID:                  orgUUID, // Store UUID, not external ID!
		UnitID:                 unitID,
		Status:                 "pending",
		IsDestroy:              isDestroy,
		Message:                message,
		PlanOnly:               planOnly,       // From CLI or config version
		AutoApply:              finalAutoApply, // Workspace default OR CLI override
		Source:                 "cli",
		IsCancelable:           true,
		CanApply:               false,
		ConfigurationVersionID: cvID,
		CreatedBy:              userID,
	}

	logger.Info("creating run",
		slog.Bool("auto_apply", finalAutoApply),
		slog.Bool("plan_only", run.PlanOnly))

	if err := h.runRepo.CreateRun(ctx, run); err != nil {
		logger.Error("failed to create run", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to create run",
			}},
		})
	}

	logger.Info("created run",
		slog.String("run_id", run.ID),
		slog.String("unit_id", unitID))

	// Create a plan for this run
	plan := &domain.TFEPlan{
		OrgID:     orgUUID,
		RunID:     run.ID,
		Status:    "pending",
		CreatedBy: userID,
	}

	if err := h.planRepo.CreatePlan(ctx, plan); err != nil {
		logger.Error("failed to create plan", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to create plan",
			}},
		})
	}

	logger.Info("created plan",
		slog.String("plan_id", plan.ID),
		slog.String("run_id", run.ID))

	// Update run with plan ID
	if err := h.runRepo.UpdateRunPlanID(ctx, run.ID, plan.ID); err != nil {
		logger.Warn("failed to update run with plan ID", slog.String("error", err.Error()))
		// Non-fatal, continue
	}

	// Trigger real plan execution asynchronously
	// Use a new context to avoid cancellation propagation
	planCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		planLogger := slog.Default().With(
			slog.String("operation", "async_plan"),
			slog.String("run_id", run.ID),
		)
		planLogger.Info("starting async plan execution")
		// Create plan executor
		executor := NewPlanExecutor(h.runRepo, h.planRepo, h.configVerRepo, h.blobStore, h.unitRepo, h.sandbox, h.runActivityRepo)

		// Execute the plan (this will run terraform plan)
		if err := executor.ExecutePlan(planCtx, run.ID); err != nil {
			planLogger.Error("plan execution failed", slog.String("error", err.Error()))
		} else {
			planLogger.Info("plan execution completed successfully")
		}
	}()

	// Return JSON:API response
	// Terraform CLI expects workspace ID in the format "ws-{uuid}"
	response := tfe.TFERun{
		ID:            run.ID,
		Status:        "planning", // Return as planning immediately
		HasChanges:    false,
		IsDestroy:     run.IsDestroy,
		Message:       run.Message,
		PlanOnly:      run.PlanOnly,
		AutoApply:     run.AutoApply,
		IsConfirmable: false,
		Actions: &tfe.RunActions{
			IsCancelable:  run.IsCancelable,
			IsConfirmable: false,
		},
		Permissions: &tfe.RunPermissions{
			CanApply: run.CanApply,
		},
		Plan: &tfe.PlanRef{
			ID: plan.ID,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: "ws-" + workspaceID, // Add ws- prefix
		},
		ConfigurationVersion: &tfe.ConfigurationVersionRef{
			ID: cvID,
		},
	}

	// For auto-apply runs, include Apply reference immediately so Terraform CLI knows to expect it
	if run.AutoApply {
		response.Apply = &tfe.ApplyRef{ID: run.ID}
		logger.Debug("added apply reference for auto-apply run", slog.String("apply_id", run.ID))
	} else {
		logger.Debug("no apply reference, user will confirm apply manually")
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusCreated)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		logger.Error("error marshaling run payload", slog.String("error", err.Error()))
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

	logger := slog.Default().With(
		slog.String("operation", "apply_run"),
		slog.String("run_id", runID),
	)

	// Get run from database
	run, err := h.runRepo.GetRun(ctx, runID)
	if err != nil {
		logger.Error("failed to get run", slog.String("error", err.Error()))
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Run %s not found", runID),
			}},
		})
	}

	// Check if run can be applied
	// Allow apply from "planned" status (waiting for confirmation)
	if run.Status != "planned" {
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

	logger.Info("triggering apply")

	// Update run status to apply_queued
	if err := h.runRepo.UpdateRunStatus(ctx, runID, "apply_queued"); err != nil {
		logger.Error("failed to update run status", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to queue apply",
			}},
		})
	}

	// Trigger real apply execution asynchronously
	// Use a new context to avoid cancellation propagation
	applyCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		applyLogger := slog.Default().With(
			slog.String("operation", "async_apply"),
			slog.String("run_id", runID),
		)
		applyLogger.Info("starting async apply execution")
		// Create apply executor
		executor := NewApplyExecutor(h.runRepo, h.planRepo, h.configVerRepo, h.blobStore, h.unitRepo, h.sandbox, h.runActivityRepo)

		// Execute the apply (this will run terraform apply)
		if err := executor.ExecuteApply(applyCtx, runID); err != nil {
			applyLogger.Error("apply execution failed", slog.String("error", err.Error()))
		} else {
			applyLogger.Info("apply execution completed successfully")
		}
	}()

	// Return updated run
	run.Status = "apply_queued"
	response := tfe.TFERun{
		ID:            run.ID,
		Status:        run.Status,
		HasChanges:    false,
		IsDestroy:     run.IsDestroy,
		Message:       run.Message,
		PlanOnly:      run.PlanOnly,
		AutoApply:     run.AutoApply,
		IsConfirmable: false,
		Actions: &tfe.RunActions{
			IsCancelable:  false,
			IsConfirmable: false,
		},
		Permissions: &tfe.RunPermissions{
			CanApply: false,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: "ws-" + run.UnitID, // Add ws- prefix
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
	logger.Debug("added apply reference", slog.String("apply_id", run.ID))

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		logger.Error("error marshaling run payload", slog.String("error", err.Error()))
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
	case "planning", "planned":
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
