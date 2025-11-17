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

	// Generate signed token for apply log streaming (same approach as plan logs)
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
	
	logData, err := h.blobStore.DownloadBlob(ctx, applyLogBlobID)
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

	// Handle offset for streaming with proper byte accounting
	// Stream format: [STX at offset 0][logText at offset 1+][ETX at offset 1+len(logText)]
	var responseData []byte

	if offsetInt == 0 {
		// First request: send STX + current logs
		responseData = append([]byte{0x02}, []byte(logText)...)
		fmt.Printf("📤 APPLY LOGS at offset=0: STX + %d bytes of log text\n", len(logText))
	} else {
		// Client already received STX (1 byte at offset 0)
		// Map stream offset to logText offset: streamOffset=1 → logText[0]
		logOffset := offsetInt - 1

		if logOffset < int64(len(logText)) {
			// Send remaining log text
			responseData = []byte(logText[logOffset:])
			fmt.Printf("📤 APPLY LOGS at offset=%d: sending %d bytes (logText[%d:])\n",
				offsetInt, len(responseData), logOffset)
		} else if logOffset == int64(len(logText)) && run.Status == "applied" {
			// All logs sent, send ETX
			responseData = []byte{0x03}
			fmt.Printf("📤 Sending ETX (End of Text) for apply %s - logs complete\n", applyID)
		} else {
			// Waiting for more logs or already sent ETX
			responseData = []byte{}
			fmt.Printf("📤 APPLY LOGS at offset=%d: no new data (waiting or complete)\n", offsetInt)
		}
	}

	c.Response().Header().Set("Content-Type", "text/plain")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write(responseData)
	return err
}
