package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func GenerateTerraformCode(appCode string, generationEndpoint string, webhookSecret string) (string, error) {

	payload := map[string]string{
		"code": appCode,
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("Error marshalling JSON: %v\n", err)
	}

	// Create request
	req, err := http.NewRequest("POST", generationEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("Error creating request: %v\n", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Secret", webhookSecret) // Replace with your webhook secret

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error making request: %v\n", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading response: %v\n", err)
	}

	// Print response
	if resp.StatusCode == 400 {
		return "", fmt.Errorf("unable to generate terraform code from the code available, is it valid application code")
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected error occured while generating code")
	}

	type GeneratorResponse struct {
		Result string `json:"result"`
		Status string `json:"status"`
	}

	var response GeneratorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("unable to parse generator response: %v", err)
	}

	return response.Result, nil

}
