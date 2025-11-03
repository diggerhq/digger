package tfe

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetPlan(c echo.Context) error {
	planID := c.Param("id")

	// This has to match whatever runID you created in POST /runs
	runID := "run-abc123"

	publicBase := os.Getenv("OPENTACO_PUBLIC_BASE_URL")
	if publicBase == "" {
		slog.Error("OPENTACO_PUBLIC_BASE_URL not set")
		return fmt.Errorf("OPENTACO_PUBLIC_BASE_URL environment variable not set")
	}


	blobID := "a-really-long-string-for-blob-id-secret"
	logsurl := fmt.Sprintf("%v/tfe/api/v2/plans/%s/logs/%s", publicBase, planID, blobID)

	plan := tfe.PlanRecord{
		ID:                   planID,
		Status:               "finished",
		ResourceAdditions:    0,
		ResourceChanges:      0,
		ResourceDestructions: 0,
		HasChanges:           false,
		LogReadURL:           logsurl,
		Run: &tfe.RunRef{
			ID: runID,
		},
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &plan); err != nil {
		fmt.Printf("error marshaling plan payload: %v\n", err)
		return err
	}
	return nil
}

func (h *TfeHandler) GetPlanLogs(c echo.Context) error {
	//TODO: verify the blobID from DB and ensure that it belongs to the right planID
	//blobID := c.Param("blobID")

	offset := c.QueryParam("offset")
	offsetInt , _ := strconv.ParseInt(offset, 10, 64)

	c.Response().Header().Set(echo.HeaderContentType, "text/plain")
	c.Response().WriteHeader(http.StatusOK)

	// Minimal realistic Terraform plan text for "no changes"
	logText := `Terraform used the selected providers to generate the following execution plan.
Resource actions are indicated with the following symbols:
  + create
  - destroy
  ~ update in-place

No changes. Your infrastructure matches the configuration.

Plan: 0 to add, 0 to change, 0 to destroy.

NOTE: This streamed logs are FAKEEEE
`

	if offsetInt > 100 {
		logText = ""
	}

	_, err := c.Response().Writer.Write([]byte(logText))
	if err != nil {
		return err
	}
	return nil
}

