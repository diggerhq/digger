package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func GenerateTerraformCode(appCode, generationEndpoint, apiToken string) (string, error) {
	slog.Debug("Generating Terraform code",
		"endpoint", generationEndpoint,
		"codeLength", len(appCode),
	)

	payload := map[string]string{
		"code": appCode,
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Error marshalling JSON for code generation", "error", err)
		return "", fmt.Errorf("Error marshalling JSON: %v\n", err)
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, generationEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Error creating request for code generation", "endpoint", generationEndpoint, "error", err)
		return "", fmt.Errorf("Error creating request: %v\n", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error making request to code generation API", "endpoint", generationEndpoint, "error", err)
		return "", fmt.Errorf("Error making request: %v\n", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading code generation API response", "error", err)
		return "", fmt.Errorf("Error reading response: %v\n", err)
	}

	// Handle non-200 responses
	if resp.StatusCode == http.StatusBadRequest {
		slog.Warn("Bad request to code generation API",
			"statusCode", resp.StatusCode,
			"response", string(body),
		)
		return "", fmt.Errorf("unable to generate terraform code from the code available, is it valid application code")
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected error from code generation API",
			"statusCode", resp.StatusCode,
			"response", string(body),
		)
		return "", fmt.Errorf("unexpected error occurred while generating code")
	}

	type GeneratorResponse struct {
		Result string `json:"result"`
		Status string `json:"status"`
	}

	var response GeneratorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		slog.Error("Unable to parse code generation response", "error", err, "response", string(body))
		return "", fmt.Errorf("unable to parse generator response: %v", err)
	}

	slog.Info("Successfully generated Terraform code",
		"status", response.Status,
		"resultLength", len(response.Result),
	)
	return response.Result, nil
}

func GetAiSummaryFromTerraformPlans(plans, summaryEndpoint, apiToken string) (string, error) {
	slog.Debug("Generating AI summary for Terraform plans",
		"endpoint", summaryEndpoint,
		"plansLength", len(plans),
	)

	payload := map[string]string{
		"terraform_plans": plans,
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Error marshalling JSON for plan summary", "error", err)
		return "", fmt.Errorf("Error marshalling JSON: %v\n", err)
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, summaryEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Error creating request for plan summary", "endpoint", summaryEndpoint, "error", err)
		return "", fmt.Errorf("Error creating request: %v\n", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error making request to summary API", "endpoint", summaryEndpoint, "error", err)
		return "", fmt.Errorf("Error making request: %v\n", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading summary API response", "error", err)
		return "", fmt.Errorf("Error reading response: %v\n", err)
	}

	// Handle non-200 responses
	if resp.StatusCode == http.StatusBadRequest {
		slog.Warn("Bad request to summary API",
			"statusCode", resp.StatusCode,
			"response", string(body),
		)
		return "", fmt.Errorf("unable to generate summary")
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected error from summary API",
			"statusCode", resp.StatusCode,
			"response", string(body),
		)
		return "", fmt.Errorf("unexpected error occurred while generating code")
	}

	type GeneratorResponse struct {
		Result string `json:"result"`
		Status string `json:"status"`
	}

	var response GeneratorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		slog.Error("Unable to parse summary response", "error", err, "response", string(body))
		return "", fmt.Errorf("unable to parse generator response: %v", err)
	}

	slog.Info("Successfully generated plan summary",
		"status", response.Status,
		"resultLength", len(response.Result),
	)
	return response.Result, nil
}
