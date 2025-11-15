package tfe

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/google/jsonapi"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetCurrentStateVersionOutputs(c echo.Context) error {
	logger := logging.FromContext(c)
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		logger.Warn("Missing workspace ID",
			"operation", "tfe_get_current_state",
		)
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)

	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		logger.Error("Failed to get org from context",
			"operation", "tfe_get_current_state",
			"workspace_id", workspaceID,
			"error", err,
		)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}

	logger.Info("Getting current state version",
		"operation", "tfe_get_current_state",
		"workspace_id", workspaceID,
		"workspace_name", workspaceName,
		"org_identifier", orgIdentifier,
	)

	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)

	// Download the state data
	stateData, err := h.directStateStore.Download(c.Request().Context(), unitUUID)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(404, map[string]string{"error": "State version not found"})
		}
		return c.JSON(500, map[string]string{"error": "Failed to download state"})
	}

	var st map[string]interface{}
	var outputs []*tfe.StateVersionOutput
	if uErr := json.Unmarshal(stateData, &st); uErr == nil {

		raw, ok := st["outputs"]
		if !ok {
			return fmt.Errorf("state version outputs not found or failed to parse")
		}

		type TfeOutputEntry struct {
			Value string `json:"value"`
			Type  string `json:"type"`
			Sensitive bool `json:"sensitive,omitempty"`
		}

		// raw is interface{} -> turn it back into JSON, then into typed map
		b, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("failed to re-marshal outputs: %w", err)
		}

		m := map[string]TfeOutputEntry{}
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("failed to unmarshal outputs as TfeOutputEntry: %w", err)
		}

		for name, v := range m {
			// v = map[string]any for this output
			b, _ := json.Marshal(v)

			out := tfe.StateVersionOutput{
				ID: uuid.NewString(),
				Name: name,
				Sensitive: v.Sensitive,
				Type: v.Type,
			}

			if err := json.Unmarshal(b, &out); err != nil {
				return nil // or return err
			}

			outputs = append(outputs, &out)
		}

	}

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, jsonapi.MediaType)
	res.WriteHeader(http.StatusOK)
	
	if err := jsonapi.MarshalPayload(res, outputs); err != nil {
		// Response already has headers; last resort is 500 with plain text
		slog.Error("failed to marshall json payload", "error", err.Error())
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to marshal jsonapi")
	}

	return c.JSON(200, map[string]interface{}{})

}
