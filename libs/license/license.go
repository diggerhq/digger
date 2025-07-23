package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

type LicenseKeyChecker struct{}

func (l LicenseKeyChecker) Check() error {
	licenseKey := os.Getenv("DIGGER_LICENSE_KEY")
	url := "https://europe-west2-prod-415611.cloudfunctions.net/licenses"
	contentType := "application/json"

	// Data to be sent in the request body
	data := map[string]string{
		"license_key": licenseKey,
	}

	// Convert the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("Error marshalling JSON for license validation", "error", err)
		return fmt.Errorf("error marshalling JSON for license validation: %v", err)
	}

	// Create a new POST request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Error creating request for license validation", "error", err)
		return fmt.Errorf("error creating request for license validation: %v", err)
	}

	// Set the content type header
	req.Header.Set("Content-Type", contentType)

	// Send the request using http.DefaultClient
	client := &http.Client{}

	slog.Debug("Sending license validation request", "url", url)
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error sending license validation request", "error", err)
		return fmt.Errorf("error sending request for license validation: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Info("License validation successful", "status", resp.StatusCode)
		return nil
	} else {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading license validation response body", "error", err)
		}
		responseText := string(bodyBytes)
		slog.Error("License validation failed",
			"status", resp.StatusCode,
			"response", responseText)
		return fmt.Errorf("license key is not valid: %v", responseText)
	}
}
