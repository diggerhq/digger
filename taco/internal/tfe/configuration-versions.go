package tfe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetConfigurationVersion(c echo.Context) error {
	ctx := c.Request().Context()
	cvID := c.Param("id")

	// Get configuration version from database
	configVer, err := h.configVerRepo.GetConfigurationVersion(ctx, cvID)
	if err != nil {
		fmt.Printf("Failed to get configuration version %s: %v\n", cvID, err)
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not found",
				"detail": fmt.Sprintf("Configuration version %s not found", cvID),
			}},
		})
	}

	// Parse status timestamps
	statusTimestamps := make(map[string]string)
	if configVer.UploadedAt != nil {
		statusTimestamps["uploaded-at"] = configVer.UploadedAt.UTC().Format(time.RFC3339)
	}

	cv := tfe.ConfigurationVersionRecord{
		ID:               cvID,
		AutoQueueRuns:    configVer.AutoQueueRuns,
		Error:            configVer.Error,
		ErrorMessage:     configVer.ErrorMessage,
		Source:           configVer.Source,
		Speculative:      configVer.Speculative,
		Status:           configVer.Status,
		StatusTimestamps: statusTimestamps,
		UploadURL:        configVer.UploadURL,
		Provisional:      configVer.Provisional,
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &cv); err != nil {
		return err
	}
	return nil
}

func (h *TfeHandler) CreateConfigurationVersions(c echo.Context) error {
	ctx := c.Request().Context()
	workspaceName := c.Param("workspace_name")

	// Get org and user context
	orgIdentifier, _ := c.Get("organization_id").(string)
	userID, _ := c.Get("user_id").(string)

	if orgIdentifier == "" {
		orgIdentifier = "default-org"
	}
	if userID == "" {
		userID = "system"
	}

	// Resolve external org ID to UUID (needed for S3 paths)
	orgUUID, err := h.identifierResolver.ResolveOrganization(ctx, orgIdentifier)
	if err != nil {
		fmt.Printf("Failed to resolve organization %s: %v\n", orgIdentifier, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to resolve organization: %v", err),
		})
	}

	// Strip ws- prefix if present to get the actual unit ID
	unitID := convertWorkspaceToStateID(workspaceName)

	publicBase := os.Getenv("OPENTACO_PUBLIC_BASE_URL")
	if publicBase == "" {
		publicBase = "http://localhost:8080" // Fallback for testing
	}

	// Parse request body to get speculative flag from CLI
	// Must manually decode JSON:API format (c.Bind doesn't work with vnd.api+json)
	var requestPayload struct {
		Data struct {
			Attributes struct {
				Speculative   *bool `json:"speculative"`
				AutoQueueRuns *bool `json:"auto-queue-runs"`
			} `json:"attributes"`
		} `json:"data"`
	}

	speculative := false // Default to false (normal apply)
	autoQueueRuns := false

	// Manually decode JSON since content-type is application/vnd.api+json
	if err := json.NewDecoder(c.Request().Body).Decode(&requestPayload); err == nil {
		if requestPayload.Data.Attributes.Speculative != nil {
			speculative = *requestPayload.Data.Attributes.Speculative
			fmt.Printf("🔍 DEBUG: CLI sent speculative=%v for workspace %s\n", speculative, workspaceName)
		} else {
			fmt.Printf("🔍 DEBUG: CLI did NOT send speculative flag for workspace %s, using default: false\n", workspaceName)
		}
		if requestPayload.Data.Attributes.AutoQueueRuns != nil {
			autoQueueRuns = *requestPayload.Data.Attributes.AutoQueueRuns
		}
	} else {
		fmt.Printf("⚠️  WARN: Failed to decode config version request body: %v\n", err)
	}

	// Create configuration version in database
	// Speculative means "plan-only" - should be false for normal apply operations
	configVer := &domain.TFEConfigurationVersion{
		OrgID:            orgUUID, // Store UUID, not external ID!
		UnitID:           unitID,
		Status:           "pending",
		Source:           "cli",
		Speculative:      speculative,   // Parse from CLI request
		AutoQueueRuns:    autoQueueRuns, // Parse from CLI request
		Provisional:      false,
		StatusTimestamps: "{}",
		CreatedBy:        userID,
	}

	if err := h.configVerRepo.CreateConfigurationVersion(ctx, configVer); err != nil {
		fmt.Printf("Failed to create configuration version: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "500",
				"title":  "internal error",
				"detail": "Failed to create configuration version",
			}},
		})
	}

	fmt.Printf("Created configuration version %s for unit %s\n", configVer.ID, unitID)

	// Generate signed upload URL
	signedUploadUrl, err := auth.SignURL(publicBase, fmt.Sprintf("/tfe/api/v2/configuration-versions/%v/upload", configVer.ID), time.Now().Add(2*time.Minute))
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG Generated upload URL: %s\n", signedUploadUrl)

	cv := tfe.ConfigurationVersionRecord{
		ID:               configVer.ID,
		AutoQueueRuns:    configVer.AutoQueueRuns,
		Error:            nil,
		ErrorMessage:     nil,
		Source:           configVer.Source,
		Speculative:      configVer.Speculative,
		Status:           configVer.Status,
		StatusTimestamps: map[string]string{},
		UploadURL:        &signedUploadUrl,
		Provisional:      configVer.Provisional,
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusCreated)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &cv); err != nil {
		fmt.Printf("error marshaling configuration version payload: %v\n", err)
		return err
	}

	return nil
}

func (h *TfeHandler) UploadConfigurationArchive(c echo.Context) error {
	ctx := c.Request().Context()
	configVersionID := c.Param("id")

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if len(body) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "empty archive"})
	}

	fmt.Printf("Received %d bytes for configuration version %s\n", len(body), configVersionID)

	// Get configuration version from database
	_, err = h.configVerRepo.GetConfigurationVersion(ctx, configVersionID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "configuration version not found"})
	}

	// Store archive in blob storage (use UploadBlob - no lock checks needed for archives)
	archiveBlobID := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVersionID)
	if err := h.blobStore.UploadBlob(ctx, archiveBlobID, body); err != nil {
		fmt.Printf("Failed to store archive: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to store archive"})
	}

	// Update configuration version status to uploaded AND set the archive blob ID
	uploadedAt := time.Now()
	if err := h.configVerRepo.UpdateConfigurationVersionStatus(ctx, configVersionID, "uploaded", &uploadedAt, &archiveBlobID); err != nil {
		fmt.Printf("Failed to update configuration version status: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update status"})
	}

	fmt.Printf("Successfully uploaded and stored archive for configuration version %s (blob: %s)\n", configVersionID, archiveBlobID)

	// 200 OK, empty body. Terraform does not expect JSON here.
	return c.NoContent(http.StatusOK)
}
