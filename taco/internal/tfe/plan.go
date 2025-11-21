package tfe

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	// Read logs from chunked S3 objects (fixed 2KB chunks)
	// Chunks are stored as plans/{planID}/chunks/00000001.log, 00000002.log, etc.
	const chunkSize = 2 * 1024 // Must match writer's chunk size

	// Determine which chunk contains the requested offset to avoid re-downloading
	// data the client already has.
	startChunk := 1
	if offsetInt > 1 { // offset includes STX byte at offset 0
		logOffset := offsetInt - 1
		startChunk = int(logOffset/chunkSize) + 1
	}

	// Number of bytes before the first chunk we fetch (used to map offsets)
	bytesBefore := int64(chunkSize * (startChunk - 1))

	var logText string
	chunkIndex := startChunk
	var fullLogs strings.Builder

	for {
		chunkKey := fmt.Sprintf("plans/%s/chunks/%08d.log", planID, chunkIndex)
		logData, err := h.blobStore.DownloadBlob(ctx, chunkKey)

		if err != nil {
			// Chunk doesn't exist - check if plan is still running
			if plan.Status == "finished" || plan.Status == "errored" {
				// Plan is done, no more chunks coming
				break
			}
			// Plan still running, this chunk doesn't exist yet
			break
		}

		// Keep chunks at full 2048 bytes (don't trim nulls) for correct offset math
		fullLogs.Write(logData)
		chunkIndex++
	}

	logText = fullLogs.String()

	// NOW trim all null bytes from the result (after offset calculation is done)
	logText = strings.TrimRight(logText, "\x00")

	// If no chunks exist yet, generate default logs based on status (only on first request)
	if logText == "" && offsetInt == 0 {
		logText = generateDefaultPlanLogs(plan)
	}

	// Handle offset for streaming with proper byte accounting
	// Stream format: [STX at offset 0][logText at offset 1+][ETX at offset 1+len(logText)]
	var responseData []byte

	if offsetInt == 0 {
		// First request: send STX + current logs
		responseData = append([]byte{0x02}, []byte(logText)...)
		fmt.Printf("📤 PLAN LOGS at offset=0: STX + %d bytes of log text\n", len(logText))
		if len(logText) > 0 {
			fmt.Printf("Log preview (first 200 chars): %.200s\n", logText)
		}
	} else {
		// Client already received STX (1 byte at offset 0)
		// Map stream offset to logText offset:
		// - stream offset 0 = STX
		// - stream offset 1 = first byte of full logs
		// We only fetched chunks starting at startChunk, so subtract bytesBefore.
		logOffset := offsetInt - 1 - bytesBefore
		if logOffset < 0 {
			logOffset = 0
		}

		if logOffset < int64(len(logText)) {
			// Send remaining log text
			responseData = []byte(logText[logOffset:])
			fmt.Printf("📤 PLAN LOGS at offset=%d: sending %d bytes (logText[%d:])\n",
				offsetInt, len(responseData), logOffset)
		} else if plan.Status == "finished" || plan.Status == "errored" {
			// All logs sent, send ETX to stop polling
			responseData = []byte{0x03}
			fmt.Printf("📤 Sending ETX (End of Text) for plan %s - logs complete\n", planID)
		} else {
			// Waiting for more logs
			responseData = []byte{}
			fmt.Printf("📤 PLAN LOGS at offset=%d: no new data (waiting or complete)\n", offsetInt)
		}
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
