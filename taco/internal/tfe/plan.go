package tfe

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetPlan(c echo.Context) error {
	ctx := c.Request().Context()
	planID := c.Param("id")

	// Get plan from database
	plan, err := h.planRepo.GetPlan(ctx, planID)
	if err != nil {
		fmt.Printf("Failed to get plan %s: %v\n", planID, err)
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Plan %s not found", planID),
			}},
		})
	}

	publicBase := os.Getenv("OPENTACO_PUBLIC_BASE_URL")
	if publicBase == "" {
		slog.Error("OPENTACO_PUBLIC_BASE_URL not set")
		return fmt.Errorf("OPENTACO_PUBLIC_BASE_URL environment variable not set")
	}

	// Generate signed token for log streaming (embedded in path, not query string)
	// This is secure because: token is time-limited, HMAC-signed, and in the path
	// (Terraform CLI strips query params but preserves path)
	// 24-hour validity to support long-running plans
	logToken, err := auth.GenerateLogStreamToken(planID, 24*time.Hour)
	if err != nil {
		fmt.Printf("Failed to generate log token: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to generate log token",
			}},
		})
	}
	logsurl := fmt.Sprintf("%s/tfe/api/v2/plans/%s/logs/%s", publicBase, planID, logToken)

	response := tfe.PlanRecord{
		ID:         plan.ID,
		Status:     plan.Status,
		LogReadURL: logsurl,
		Run: &tfe.RunRef{
			ID: plan.RunID,
		},
	}

	// Only include resource counts when plan is finished
	// If we send HasChanges:false before the plan completes, Terraform CLI
	// will think there's nothing to apply and won't prompt for confirmation!
	if plan.Status == "finished" {
		response.ResourceAdditions = plan.ResourceAdditions
		response.ResourceChanges = plan.ResourceChanges
		response.ResourceDestructions = plan.ResourceDestructions
		response.HasChanges = plan.HasChanges
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		fmt.Printf("error marshaling plan payload: %v\n", err)
		return err
	}
	return nil
}

func (h *TfeHandler) GetPlanLogs(c echo.Context) error {
	ctx := c.Request().Context()
	planID := c.Param("planID")
	logToken := c.Param("token")

	// Verify the log streaming token
	if !auth.VerifyLogStreamToken(logToken, planID) {
		fmt.Printf("Invalid log stream token for plan %s\n", planID)
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired log token"})
	}

	offset := c.QueryParam("offset")
	offsetInt, _ := strconv.ParseInt(offset, 10, 64)

	// Get plan from database
	plan, err := h.planRepo.GetPlan(ctx, planID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "plan not found"})
	}
	responseData, err := streamChunkedLogs(ctx, h.blobStore, logStreamOptions{
		Prefix:    "plans",
		Label:     "PLAN",
		ID:        planID,
		Offset:    offsetInt,
		ChunkSize: 2 * 1024,
		GenerateDefaultText: func() string {
			return generateDefaultPlanLogs(plan)
		},
		IsComplete: func() bool {
			return plan.Status == "finished" || plan.Status == "errored"
		},
		AppendETXOnFirst: false,
	})
	if err != nil {
		return err
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/plain")
	c.Response().WriteHeader(http.StatusOK)

	_, err = c.Response().Writer.Write(responseData)
	return err
}

// GetPlanJSONOutput returns the structured JSON representation of a plan
// This is what Terraform CLI uses to decide whether to show the confirmation dialog
func (h *TfeHandler) GetPlanJSONOutput(c echo.Context) error {
	ctx := c.Request().Context()
	planID := c.Param("id")

	// Get plan from database
	plan, err := h.planRepo.GetPlan(ctx, planID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Plan %s not found", planID),
			}},
		})
	}

	// Return a minimal Terraform JSON plan format
	// This is the structured representation that the CLI parses to determine if it should prompt
	jsonPlan := map[string]interface{}{
		"format_version":    "1.2",
		"terraform_version": "1.9.0",
		"resource_changes":  []interface{}{}, // Empty for now, but signals that plan exists
	}

	// If plan is finished, include resource change summary
	if plan.Status == "finished" {
		// Create dummy resource changes based on our counts
		// The CLI checks if this array has entries to decide whether to prompt
		resourceChanges := make([]interface{}, 0)

		// Add placeholder entries for additions
		for i := 0; i < plan.ResourceAdditions; i++ {
			resourceChanges = append(resourceChanges, map[string]interface{}{
				"change": map[string]interface{}{
					"actions": []string{"create"},
				},
			})
		}

		// Add placeholder entries for changes
		for i := 0; i < plan.ResourceChanges; i++ {
			resourceChanges = append(resourceChanges, map[string]interface{}{
				"change": map[string]interface{}{
					"actions": []string{"update"},
				},
			})
		}

		// Add placeholder entries for destructions
		for i := 0; i < plan.ResourceDestructions; i++ {
			resourceChanges = append(resourceChanges, map[string]interface{}{
				"change": map[string]interface{}{
					"actions": []string{"delete"},
				},
			})
		}

		jsonPlan["resource_changes"] = resourceChanges
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().WriteHeader(http.StatusOK)
	return c.JSON(http.StatusOK, jsonPlan)
}

func generateDefaultPlanLogs(plan *domain.TFEPlan) string {
	// Don't show resource counts in logs until plan is finished
	// Terraform CLI parses the logs to determine if changes exist!
	if plan.Status == "finished" {
		return fmt.Sprintf(`Terraform used the selected providers to generate the following execution plan.
Resource actions are indicated with the following symbols:
  + create
  - destroy
  ~ update in-place

Plan: %d to add, %d to change, %d to destroy.
`, plan.ResourceAdditions, plan.ResourceChanges, plan.ResourceDestructions)
	}

	// While planning, return EMPTY - don't send any text that CLI might parse as a plan summary!
	// The CLI will keep polling until it gets real content.
	return ""
}
