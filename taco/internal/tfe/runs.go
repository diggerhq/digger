package tfe

import (
	"fmt"
	"net/http"

	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)


func (h *TfeHandler) GetRun(c echo.Context) error {
	runID := c.Param("id")

	// You should look these up from storage in a real impl.
	// For now we're going to hardcode stable IDs that match what you
	// returned from POST /runs.
	planID := "plan-xyz789"
	workspaceID := "ws-0b44c5b1-8321-43e3-864d-a2921d004835"
	cvID := "cv-1234567890"

	run := tfe.TFERun{
		ID:         runID,
		Status:     "planned_and_finished",
		IsDestroy:  false,
		Message:    "Queued manually via Terraform CLI",
		PlanOnly:   true,
		Actions: &tfe.RunActions{
			IsCancelable: false,
			CanApply:     false,
		},
		Plan: &tfe.PlanRef{
			ID: planID,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: workspaceID,
		},
		ConfigurationVersion: &tfe.ConfigurationVersionRef{
			ID: cvID,
		},
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &run); err != nil {
		fmt.Printf("error marshaling run payload: %v\n", err)
		return err
	}
	return nil
}


func (h *TfeHandler) CreateRun(c echo.Context) error {
	// You could decode the incoming JSON here to read workspace ID,
	// configuration-version ID, message, plan-only, etc.
	// For now we’ll just hardcode / stub.
	workspaceID := "ws-0b44c5b1-8321-43e3-864d-a2921d004835"
	cvID := "cv-1234567890"

	runID := "run-abc123"
	planID := "plan-xyz789"

	run := tfe.TFERun{
		ID:         runID,
		Status:     "planning", // Terraform will expect to poll until it becomes "planned_and_finished"
		IsDestroy:  false,
		Message:    "Queued manually via Terraform CLI",
		PlanOnly:   true,
		Actions: &tfe.RunActions{
			IsCancelable: true,
			CanApply:     false,
		},
		Plan: &tfe.PlanRef{
			ID: planID,
		},
		Workspace: &tfe.WorkspaceRef{
			ID: workspaceID,
		},
		ConfigurationVersion: &tfe.ConfigurationVersionRef{
			ID: cvID,
		},
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusCreated)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &run); err != nil {
		fmt.Printf("error marshaling run payload: %v\n", err)
		return err
	}

	return nil
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