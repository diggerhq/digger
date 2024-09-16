package backendapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/terraform_utils"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

type NoopApi struct {
}

func (n NoopApi) ReportProject(namespace string, projectName string, configurationYaml string) error {
	return nil
}

func (n NoopApi) ReportProjectRun(namespace string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error {
	return nil
}

func (n NoopApi) ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *terraform_utils.TerraformSummary, planJson string, PrCommentUrl string, terraformOutput string) (*scheduler.SerializedBatch, error) {
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
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = filepath.Join(u.Path, "repos", namespace, "report-projects")

	request := map[string]interface{}{
		"name":              projectName,
		"configurationYaml": configurationYaml,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Not able to marshal request: %v", err)
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

func (d DiggerApi) ReportProjectRun(namespace string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error {
	u, err := url.Parse(d.DiggerHost)
	if err != nil {
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}

	u.Path = filepath.Join(u.Path, "repos", namespace, "projects", projectName, "runs")

	request := map[string]interface{}{
		"startedAt": startedAt,
		"endedAt":   endedAt,
		"status":    status,
		"command":   command,
		"output":    output,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Not able to marshal request: %v", err)
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
		return fmt.Errorf("unexpected status when reporting a project run: %v", resp.StatusCode)
	}

	return nil
}

func (d DiggerApi) ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *terraform_utils.TerraformSummary, planJson string, PrCommentUrl string, terraformOutput string) (*scheduler.SerializedBatch, error) {
	u, err := url.Parse(d.DiggerHost)
	if err != nil {
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}

	var planSummaryJson interface{}
	var planFootprint *terraform_utils.TerraformPlanFootprint = &terraform_utils.TerraformPlanFootprint{}
	if summary == nil {
		log.Printf("Warning: nil passed to plan result, sending empty")
		planSummaryJson = nil
		planFootprint = nil
	} else {
		planSummary := summary
		planSummaryJson = planSummary.ToJson()
		if planJson != "" {
			planFootprint, err = terraform_utils.GetPlanFootprint(planJson)
			if err != nil {
				log.Printf("Error, could not get footprint from json plan: %v", err)
			}
		}
	}

	u.Path = filepath.Join(u.Path, "repos", repo, "projects", projectName, "jobs", jobId, "set-status")
	request := map[string]interface{}{
		"status":             status,
		"timestamp":          timestamp,
		"job_summary":        planSummaryJson,
		"job_plan_footprint": planFootprint.ToJson(),
		"pr_comment_url":     PrCommentUrl,
		"terraform_output":   terraformOutput,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Not able to marshal request: %v", err)
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
		fmt.Println("error opening file:", err)
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
		fmt.Println("Error creating form file:", err)
		return nil, nil, fmt.Errorf("error creating form file: %v", err)
	}

	// Copy the file content to the form file writer
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		fmt.Println("Error copying file content:", err)
		return nil, nil, fmt.Errorf("error copying file content: %v", err)
	}

	// Close the multipart writer to finalize the request body
	multipartWriter.Close()

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", uploadUrl, &requestBody)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set the content type header
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", d.AuthToken))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return nil, nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read and print the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
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
		log.Printf("failed to create url: %v", err)
		return nil, err
	}

	// Create a new HTTP request
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
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
		log.Println("WARNING: running in 'backendless' mode. Features that require backend will not be available.")
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
