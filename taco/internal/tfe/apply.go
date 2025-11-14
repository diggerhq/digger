package tfe

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

// GetApply returns details about a specific apply
func (h *TfeHandler) GetApply(c echo.Context) error {
	ctx := c.Request().Context()
	applyID := c.Param("id")

	// Get the apply from database (for now, we derive apply from run)
	// In future, we could have a separate TFEApply table
	run, err := h.runRepo.GetRun(ctx, applyID) // Using apply ID as run ID for simplicity
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Apply %s not found", applyID),
			}},
		})
	}

	publicBase := os.Getenv("OPENTACO_PUBLIC_BASE_URL")
	if publicBase == "" {
		return fmt.Errorf("OPENTACO_PUBLIC_BASE_URL environment variable not set")
	}

	// Generate signed token for apply log streaming
	logToken, err := auth.GenerateLogStreamToken(applyID, 24*time.Hour)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to generate log token",
			}},
		})
	}
	logsurl := fmt.Sprintf("%s/tfe/api/v2/applies/%s/logs/%s", publicBase, applyID, logToken)

	// Determine apply status based on run status
	applyStatus := "pending"
	switch run.Status {
	case "applying":
		applyStatus = "running"
	case "applied":
		applyStatus = "finished"
	case "errored":
		applyStatus = "errored"
	}

	response := tfe.ApplyRecord{
		ID:         applyID,
		Status:     applyStatus,
		LogReadURL: logsurl,
		Run: &tfe.RunRef{
			ID: run.ID,
		},
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &response); err != nil {
		fmt.Printf("error marshaling apply payload: %v\n", err)
		return err
	}
	return nil
}

// GetApplyLogs streams apply logs to the Terraform CLI
func (h *TfeHandler) GetApplyLogs(c echo.Context) error {
	ctx := c.Request().Context()
	applyID := c.Param("applyID")
	logToken := c.Param("token")

	// Verify the log streaming token
	if !auth.VerifyLogStreamToken(logToken, applyID) {
		fmt.Printf("Invalid log stream token for apply %s\n", applyID)
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired log token"})
	}

	offset := c.QueryParam("offset")
	offsetInt, _ := strconv.ParseInt(offset, 10, 64)

	// Get run (apply ID is the same as run ID in our simplified model)
	run, err := h.runRepo.GetRun(ctx, applyID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "apply not found"})
	}

	// Try to get apply logs from blob storage
	var logText string
	applyLogBlobID := fmt.Sprintf("runs/%s/apply-logs.txt", run.ID)
	
	logData, err := h.blobStore.Download(ctx, applyLogBlobID)
	if err == nil {
		logText = string(logData)
	} else {
		// If logs don't exist yet, return placeholder
		if run.Status == "applying" || run.Status == "apply_queued" {
			logText = "Waiting for apply to start...\n"
		} else {
			logText = "Apply logs not available\n"
		}
	}

	// Handle offset for streaming
	if offsetInt > 0 && offsetInt < int64(len(logText)) {
		logText = logText[offsetInt:]
	} else if offsetInt >= int64(len(logText)) {
		logText = ""
	}

	c.Response().Header().Set("Content-Type", "text/plain")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write([]byte(logText))
	return err
}

