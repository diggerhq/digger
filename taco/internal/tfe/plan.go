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
		ID:                   plan.ID,
		Status:               plan.Status,
		ResourceAdditions:    plan.ResourceAdditions,
		ResourceChanges:      plan.ResourceChanges,
		ResourceDestructions: plan.ResourceDestructions,
		HasChanges:           plan.HasChanges,
		LogReadURL:           logsurl,
		Run: &tfe.RunRef{
			ID: plan.RunID,
		},
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

	// Check if logs exist in blob storage
	var logText string
	if plan.LogBlobID != nil {
		// Try to get logs from blob storage
		logData, err := h.blobStore.Download(ctx, *plan.LogBlobID)
		if err != nil {
			fmt.Printf("Failed to get logs from blob storage: %v\n", err)
			// Fall back to default logs
			logText = generateDefaultPlanLogs(plan)
		} else {
			logText = string(logData)
		}
	} else {
		// Generate default logs based on plan status
		logText = generateDefaultPlanLogs(plan)
	}

	// Handle offset for streaming (TFE streams logs incrementally)
	if offsetInt > 0 && offsetInt < int64(len(logText)) {
		logText = logText[offsetInt:]
	} else if offsetInt >= int64(len(logText)) {
		logText = "" // No new data
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/plain")
	c.Response().WriteHeader(http.StatusOK)

	_, err = c.Response().Writer.Write([]byte(logText))
	return err
}

func generateDefaultPlanLogs(plan *domain.TFEPlan) string {
	return fmt.Sprintf(`Terraform used the selected providers to generate the following execution plan.
Resource actions are indicated with the following symbols:
  + create
  - destroy
  ~ update in-place

Plan: %d to add, %d to change, %d to destroy.

Status: %s
`, plan.ResourceAdditions, plan.ResourceChanges, plan.ResourceDestructions, plan.Status)
}

