package backendapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/comment_utils"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/diggerhq/digger/libs/iac_utils"
	"github.com/diggerhq/digger/libs/scheduler"
)

type NoopApi struct {
}

func (n NoopApi) ReportProject(namespace string, projectName string, configurationYaml string) error {
	return nil
}

func (n NoopApi) ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *iac_utils.IacSummary, planJson string, PrCommentUrl string, PrCommentId string, terraformOutput string, iacUtils iac_utils.IacUtils) (*scheduler.SerializedBatch, error) {
	return nil, nil
}

func (n NoopApi) UploadJobArtefact(zipLocation string) (*int, *string, error) {
	return nil, nil, nil
}

func (n NoopApi) DownloadJobArtefact(downloadTo string) (*string, error) {
	return nil, nil
}

type DiggerApi struct {
	DiggerHost string
	AuthToken  string
	HttpClient *http.Client
}

func (d DiggerApi) ReportProject(namespace string, projectName string, configurationYaml string) error {
	u, err := url.Parse(d.DiggerHost)
	if err != nil {
		slog.Error("not able to parse digger cloud url", "error", err)
		return fmt.Errorf("not able to parse digger cloud url: %v", err)
	}
	u.Path = filepath.Join(u.Path, "repos", namespace, "report-projects")

	request := map[string]interface{}{
		"name":              projectName,
		"configurationYaml": configurationYaml,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		slog.Error("not able to marshal request", "error", err)
		return fmt.Errorf("not able to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonData))

	if err != nil {
		return fmt.Errorf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.AuthToken))

	resp, err := d.HttpClient.Do(req)

	if err != nil {
		return fmt.Errorf("error while sending request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status when reporting a project: %v", resp.StatusCode)
	}

	return nil
}

func (d DiggerApi) ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *iac_utils.IacSummary, planJson string, PrCommentUrl string, PrCommentId string, terraformOutput string, iacUtils iac_utils.IacUtils) (*scheduler.SerializedBatch, error) {
	repoNameForBackendReporting := strings.ReplaceAll(repo, "/", "-")
	u, err := url.Parse(d.DiggerHost)
	if err != nil {
		slog.Error("not able to parse digger cloud url", "error", err)
		return nil, fmt.Errorf("not able to parse digger cloud url: %v", err)
	}

	var planSummaryJson interface{}
	var planFootprint = &iac_utils.IacPlanFootprint{}
	if summary == nil {
		slog.Warn("warning: nil passed to plan result, sending empty")
		planSummaryJson = nil
		planFootprint = nil
	} else {
		planSummary := summary
		planSummaryJson = planSummary.ToJson()
		if planJson != "" {
			planFootprint, err = iacUtils.GetPlanFootprint(planJson)
			if err != nil {
				slog.Error("error, could not get footprint from json plan", "error", err)
			}
		}
	}

	workflowUrl := comment_utils.GetWorkflowUrl()
	u.Path = filepath.Join(u.Path, "repos", repoNameForBackendReporting, "projects", projectName, "jobs", jobId, "set-status")
	request := map[string]interface{}{
		"status":             status,
		"timestamp":          timestamp,
		"job_summary":        planSummaryJson,
		"job_plan_footprint": planFootprint.ToJson(),
		"pr_comment_url":     PrCommentUrl,
		"pr_comment_id":      PrCommentId,
		"terraform_output":   terraformOutput,
		"workflow_url":       workflowUrl,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		slog.Error("not able to marshal request", "error", err)
		return nil, fmt.Errorf("not able to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonData))

	if err != nil {
		return nil, fmt.Errorf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.AuthToken))

	resp, err := d.HttpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("error while sending request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status when reporting a project job status: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %v", err)
	}

	var response scheduler.SerializedBatch
	json.Unmarshal(body, &response)

	return &response, nil
}

func (d DiggerApi) UploadJobArtefact(zipLocation string) (*int, *string, error) {
	u, err := url.Parse(d.DiggerHost)
	if err != nil {
		return nil, nil, err
	}
	u.Path = path.Join(u.Path, "job_artefacts")
	uploadUrl := u.String()
	filePath := zipLocation

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("error opening file", "error", err)
		return nil, nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Create a buffer to store our request body as bytes
	var requestBody bytes.Buffer

	// Create a multipart writer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Create a form file writer for our file field
	fileWriter, err := multipartWriter.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		slog.Error("error creating form file", "error", err)
		return nil, nil, fmt.Errorf("error creating form file: %v", err)
	}

	// Copy the file content to the form file writer
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		slog.Error("error copying file content", "error", err)
		return nil, nil, fmt.Errorf("error copying file content: %v", err)
	}

	// Close the multipart writer to finalize the request body
	multipartWriter.Close()

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", uploadUrl, &requestBody)
	if err != nil {
		slog.Error("error creating request", "error", err)
		return nil, nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set the content type header
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", d.AuthToken))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("error sending request", "error", err)
		return nil, nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read and print the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("error reading response", "error", err)
		return nil, nil, fmt.Errorf("error reading response: %v", err)
	}

	b := string(body)
	return &resp.StatusCode, &b, nil
}

func getFilename(resp *http.Response) string {
	// Check the Content-Disposition header
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if filename, ok := params["filename"]; ok {
				return filename
			}
		}
	}
	// Fallback to the last part of the URL path
	return path.Base(resp.Request.URL.Path)
}

func (d DiggerApi) DownloadJobArtefact(downloadTo string) (*string, error) {
	// Download the zip file
	downloadUrl, err := url.JoinPath(d.DiggerHost, "job_artefacts")
	if err != nil {
		slog.Error("failed to create url", "error", err)
		return nil, err
	}

	// Create a new HTTP request
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		slog.Error("error creating request", "error", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set the content type header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", d.AuthToken))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download zip: %w", err)
	}
	defer resp.Body.Close()

	// Create a temporary file to store the zip
	tempZipFile, err := os.Create(path.Join(downloadTo, getFilename(resp)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip file: %w", err)
	}
	defer tempZipFile.Close()

	// Copy the downloaded content to the temporary file
	_, err = io.Copy(tempZipFile, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to save zip content: %w", err)
	}

	// note that fileName include absolute path to the zip file
	fileName := tempZipFile.Name()
	return &fileName, nil
}

func NewBackendApi(hostName string, authToken string) Api {
	var backendApi Api
	if os.Getenv("NO_BACKEND") == "true" {
		slog.Warn("running in 'backendless' mode - features that require backend will not be available")
		backendApi = NoopApi{}
	} else {
		backendApi = DiggerApi{
			DiggerHost: hostName,
			AuthToken:  authToken,
			HttpClient: http.DefaultClient,
		}
	}
	return backendApi
}
