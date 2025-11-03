package tfe

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetConfigurationVersion(c echo.Context) error {
	cvID := c.Param("id")

	// In a real impl you'd look this up from memory/db. For now assume we stored:
	// - that cvID exists
	// - upload already happened
	// We'll fake a timestamp.
	uploadedAt := time.Now().UTC().Format(time.RFC3339)

	cv := tfe.ConfigurationVersionRecord{
		ID:            cvID,
		AutoQueueRuns: false,
		Error:         nil,
		ErrorMessage:  nil,
		Source:        "cli",
		Speculative:   true,
		Status:        "uploaded", // <-- important change
		StatusTimestamps: map[string]string{
			"uploaded-at": uploadedAt,
		},
		UploadURL:        nil,     // <-- becomes null in JSON now
		Provisional:      false,
		IngressAttributes: nil,    // emit relationships.ingress-attributes.data = null
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().WriteHeader(http.StatusOK)

	if err := jsonapi.MarshalPayload(c.Response().Writer, &cv); err != nil {
		return err
	}
	return nil
}

func (h *TfeHandler) CreateConfigurationVersions(c echo.Context) error {
	cvId := "cv-1234567890"
	publicBase := os.Getenv("OPENTACO_PUBLIC_BASE_URL")
	if publicBase == "" {
		slog.Error("OPENTACO_PUBLIC_BASE_URL environment variable not set")
		return fmt.Errorf("OPENTACO_PUBLIC_BASE_URL environment variable not set")
	}
	signedUploadUrl, err := auth.SignURL(publicBase, fmt.Sprintf("/tfe/api/v2/configuration-versions/%v/upload", cvId), time.Now().Add(2*time.Minute))
	if err != nil {
		return err
	}
	cv := tfe.ConfigurationVersionRecord{
		ID:               cvId,
		AutoQueueRuns:    false, // you can choose true/false; docs default true
		Error:            nil,
		ErrorMessage:     nil,
		Source:           "cli",       // HashiCorp examples show "tfe-api" or "gitlab"; "cli" is fine for CLI-driven runs.
		Speculative:      true,        // for terraform plan in remote mode
		Status:           "pending",   // initial status according to docs
		StatusTimestamps: map[string]string{},
		UploadURL:        &signedUploadUrl,
		Provisional:      false,
		IngressAttributes: nil,
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
	configVersionID := c.Param("configVersionID")

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	fmt.Printf("received %d bytes for %s\n", len(body), configVersionID)

	// 200 OK, empty body. Terraform does not expect JSON here.
	return c.NoContent(http.StatusOK)
}