package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type LicenseKeyChecker struct {
}

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
		log.Println("Error marshalling JSON:", err)
		return fmt.Errorf("error marhsalling json for license validation: %v", err)
	}

	// Create a new POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Error creating request:", err)
		return fmt.Errorf("error creating request for license validation: %v", err)
	}

	// Set the content type header
	req.Header.Set("Content-Type", contentType)

	// Send the request using http.DefaultClient
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request:", err)
		return fmt.Errorf("error sending request for license validation: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	} else {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("error while reading response body")
		}
		return fmt.Errorf("license key is not valid: %v", string(bodyBytes))
	}

}
