package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/diggerhq/digger/libs/locking/gcp"
	"github.com/google/go-github/v61/github"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type GithubPlanStorage struct {
	Client            *github.Client
	Owner             string
	RepoName          string
	PullRequestNumber int
	ZipManager        Zipper
}

func (gps *GithubPlanStorage) StorePlanFile(fileContents []byte, artifactName string, storedPlanFilePath string) error {
	slog.Debug("Storing plan file in GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", artifactName,
		"path", storedPlanFilePath,
		"size", len(fileContents))

	actionsRuntimeToken := os.Getenv("ACTIONS_RUNTIME_TOKEN")
	actionsRuntimeURL := os.Getenv("ACTIONS_RUNTIME_URL")
	githubRunID := os.Getenv("GITHUB_RUN_ID")
	artifactBase := fmt.Sprintf("%s_apis/pipelines/workflows/%s/artifacts?api-version=6.0-preview", actionsRuntimeURL, githubRunID)

	headers := map[string]string{
		"Accept":        "application/json;api-version=6.0-preview",
		"Authorization": "Bearer " + actionsRuntimeToken,
		"Content-Type":  "application/json",
	}

	// Create Artifact
	createArtifactURL := artifactBase
	createArtifactData := map[string]string{"type": "actions_storage", "name": artifactName}
	createArtifactBody, _ := json.Marshal(createArtifactData)

	slog.Debug("Creating GitHub artifact", "url", createArtifactURL, "name", artifactName)
	createArtifactResponse, err := doRequest("POST", createArtifactURL, headers, createArtifactBody)
	if createArtifactResponse == nil || err != nil {
		slog.Error("Failed to create GitHub artifact",
			"error", err,
			"artifactName", artifactName)
		return fmt.Errorf("could not create artifact with github %v", err)
	}
	defer createArtifactResponse.Body.Close()

	// Extract Resource URL
	createArtifactResponseBody, _ := io.ReadAll(createArtifactResponse.Body)
	var createArtifactResponseMap map[string]interface{}
	json.Unmarshal(createArtifactResponseBody, &createArtifactResponseMap)
	resourceURL := createArtifactResponseMap["fileContainerResourceUrl"].(string)

	// Upload Data
	uploadURL := fmt.Sprintf("%s?itemPath=%s/%s", resourceURL, artifactName, storedPlanFilePath)
	uploadData := fileContents
	dataLen := len(uploadData)
	headers["Content-Type"] = "application/octet-stream"
	headers["Content-Range"] = fmt.Sprintf("bytes 0-%v/%v", dataLen-1, dataLen)

	slog.Debug("Uploading file to GitHub artifact",
		"url", uploadURL,
		"size", dataLen)
	_, err = doRequest("PUT", uploadURL, headers, uploadData)
	if err != nil {
		slog.Error("Failed to upload file to GitHub artifact",
			"error", err,
			"artifactName", artifactName)
		return fmt.Errorf("could not upload artifact file %v", err)
	}

	// Update Artifact Size
	headers = map[string]string{
		"Accept":        "application/json;api-version=6.0-preview",
		"Authorization": "Bearer " + actionsRuntimeToken,
		"Content-Type":  "application/json",
	}
	updateArtifactURL := fmt.Sprintf("%s&artifactName=%s", artifactBase, artifactName)
	updateArtifactData := map[string]int{"size": dataLen}
	updateArtifactBody, _ := json.Marshal(updateArtifactData)

	slog.Debug("Finalizing GitHub artifact upload",
		"url", updateArtifactURL,
		"size", dataLen)
	_, err = doRequest("PATCH", updateArtifactURL, headers, updateArtifactBody)
	if err != nil {
		slog.Error("Failed to finalize GitHub artifact upload",
			"error", err,
			"artifactName", artifactName)
		return fmt.Errorf("could finalize artifact upload: %v", err)
	}

	slog.Info("Successfully stored plan file in GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", artifactName,
		"size", dataLen)
	return nil
}

func doRequest(method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		slog.Error("Error creating request",
			"method", method,
			"url", url,
			"error", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error making request",
			"method", method,
			"url", url,
			"error", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("Request failed",
			"method", method,
			"url", url,
			"statusCode", resp.StatusCode,
			"response", string(respBody))
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}
	return resp, nil
}

func (gps *GithubPlanStorage) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	slog.Debug("Retrieving plan from GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", artifactName,
		"localPath", localPlanFilePath)

	plansFilename, err := gps.DownloadLatestPlans(artifactName)

	if err != nil {
		slog.Error("Failed to download plan from GitHub artifacts",
			"error", err,
			"artifactName", artifactName)
		return nil, fmt.Errorf("error downloading plan: %v", err)
	}

	if plansFilename == "" {
		slog.Error("No plans found for this PR",
			"owner", gps.Owner,
			"repo", gps.RepoName,
			"prNumber", gps.PullRequestNumber)
		return nil, fmt.Errorf("no plans found for this PR")
	}

	plansFilename, err = gps.ZipManager.GetFileFromZip(plansFilename, localPlanFilePath)

	if err != nil {
		slog.Error("Failed to extract plan from zip",
			"error", err,
			"zipFile", plansFilename,
			"outputPath", localPlanFilePath)
		return nil, fmt.Errorf("error extracting plan: %v", err)
	}

	slog.Info("Successfully retrieved plan from GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", artifactName,
		"localPath", plansFilename)
	return &plansFilename, nil
}

func (gps *GithubPlanStorage) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	slog.Debug("Checking if plan exists in GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", artifactName,
		"path", storedPlanFilePath)

	artifacts, _, err := gps.Client.Actions.ListArtifacts(context.Background(), gps.Owner, gps.RepoName, &github.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		slog.Error("Failed to list GitHub artifacts",
			"error", err,
			"owner", gps.Owner,
			"repo", gps.RepoName)
		return false, err
	}

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, artifactName)

	if latestPlans == nil {
		slog.Debug("Plan does not exist in GitHub artifacts",
			"owner", gps.Owner,
			"repo", gps.RepoName,
			"artifactName", artifactName)
		return false, nil
	}

	slog.Debug("Plan exists in GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", artifactName,
		"artifactId", *latestPlans.ID,
		"createdAt", latestPlans.CreatedAt.Time,
		"updatedAt", latestPlans.UpdatedAt.Time)
	return true, nil
}

func (gps *GithubPlanStorage) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	// GitHub artifacts can't be deleted via API, they expire automatically
	slog.Debug("GitHub artifacts cannot be deleted, they expire automatically",
		"artifactName", artifactName,
		"path", storedPlanFilePath)
	return nil
}

func (gps *GithubPlanStorage) DownloadLatestPlans(storedPlanFilePath string) (string, error) {
	slog.Debug("Downloading latest plans from GitHub artifacts",
		"owner", gps.Owner,
		"repo", gps.RepoName,
		"artifactName", storedPlanFilePath)

	artifacts, _, err := gps.Client.Actions.ListArtifacts(context.Background(), gps.Owner, gps.RepoName, &github.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		slog.Error("Failed to list GitHub artifacts",
			"error", err,
			"owner", gps.Owner,
			"repo", gps.RepoName)
		return "", err
	}

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, storedPlanFilePath)

	if latestPlans == nil {
		slog.Debug("No matching artifacts found",
			"artifactName", storedPlanFilePath)
		return "", nil
	}

	downloadUrl, _, err := gps.Client.Actions.DownloadArtifact(context.Background(), gps.Owner, gps.RepoName, *latestPlans.ID, 0)

	if err != nil {
		slog.Error("Failed to get download URL for artifact",
			"error", err,
			"artifactId", *latestPlans.ID)
		return "", err
	}
	filename := storedPlanFilePath + ".zip"

	slog.Debug("Downloading artifact to file",
		"url", downloadUrl.String(),
		"outputFile", filename)
	err = downloadArtifactIntoFile(downloadUrl, filename)

	if err != nil {
		slog.Error("Failed to download artifact to file",
			"error", err,
			"url", downloadUrl.String(),
			"outputFile", filename)
		return "", err
	}

	slog.Info("Successfully downloaded plans from GitHub artifacts",
		"artifactId", *latestPlans.ID,
		"outputFile", filename,
		"size", *latestPlans.SizeInBytes)
	return filename, nil
}

func downloadArtifactIntoFile(artifactUrl *url.URL, outputFile string) error {
	slog.Debug("Executing wget to download artifact",
		"url", artifactUrl.String(),
		"outputFile", outputFile)

	cmd := exec.Command("wget", "-O", outputFile, artifactUrl.String())
	output, err := cmd.Output()
	if err != nil {
		slog.Error("wget command failed",
			"error", err,
			"output", string(output))
		return err
	}

	slog.Info("Successfully fetched plan artifact", "outputFile", outputFile)
	return nil
}

func getLatestArtifactWithName(artifacts []*github.Artifact, name string) *github.Artifact {
	var latest *github.Artifact

	for _, item := range artifacts {
		if *item.Name != name {
			continue
		}
		if latest == nil || item.UpdatedAt.Time.After(latest.UpdatedAt.Time) {
			latest = item
		}
	}

	if latest != nil {
		slog.Debug("Found latest artifact with name",
			"name", name,
			"id", *latest.ID,
			"updatedAt", latest.UpdatedAt.Time)
	} else {
		slog.Debug("No artifact found with name", "name", name)
	}

	return latest
}

// TODO: refactor this function to make it fully parametarised and no reliance on env variables
func NewPlanStorage(ghToken string, ghRepoOwner string, ghRepositoryName string, prNumber *int) (PlanStorage, error) {
	var planStorage PlanStorage

	uploadDestination := strings.ToLower(os.Getenv("PLAN_UPLOAD_DESTINATION"))
	slog.Info("Initializing plan storage",
		"destination", uploadDestination,
		"owner", ghRepoOwner,
		"repo", ghRepositoryName)

	switch {
	case uploadDestination == "github":
		if ghToken == "" {
			slog.Error("GITHUB_TOKEN not specified for GitHub plan storage")
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		zipManager := Zipper{}
		slog.Debug("Using GitHub artifacts for plan storage")
		planStorage = &GithubPlanStorage{
			Client:            github.NewTokenClient(context.Background(), ghToken),
			Owner:             ghRepoOwner,
			RepoName:          ghRepositoryName,
			PullRequestNumber: *prNumber,
			ZipManager:        zipManager,
		}
	case uploadDestination == "gcp":
		ctx, client := gcp.GetGoogleStorageClient()
		bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET"))
		if bucketName == "" {
			slog.Error("GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET not defined for GCP plan storage")
			return nil, fmt.Errorf("GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET is not defined")
		}
		bucket := client.Bucket(bucketName)
		slog.Debug("Using GCP storage for plan storage", "bucket", bucketName)
		planStorage = &PlanStorageGcp{
			Client:  client,
			Bucket:  bucket,
			Context: ctx,
		}
	case uploadDestination == "aws":
		bucketName := strings.ToLower(os.Getenv("AWS_S3_BUCKET"))
		encryptionEnabled := os.Getenv("PLAN_UPLOAD_S3_ENCRYPTION_ENABLED") == "true"
		encryptionType := os.Getenv("PLAN_UPLOAD_S3_ENCRYPTION_TYPE")
		encryptionKmsId := os.Getenv("PLAN_UPLOAD_S3_ENCRYPTION_KMS_ID")

		slog.Debug("Using AWS S3 for plan storage",
			"bucket", bucketName,
			"encryptionEnabled", encryptionEnabled,
			"encryptionType", encryptionType)

		var err error
		planStorage, err = NewAWSPlanStorage(bucketName, encryptionEnabled, encryptionType, encryptionKmsId)
		if err != nil {
			slog.Error("Failed to create AWS plan storage", "error", err)
			return nil, fmt.Errorf("error while creating AWS plan storage: %v", err)
		}
	case uploadDestination == "gitlab":
		slog.Warn("GitLab plan storage not yet implemented")
		//TODO implement me
	case uploadDestination == "azure":
		containerName := strings.ToLower(os.Getenv("AZURE_STORAGE_CONTAINER"))
		if containerName == "" {
			slog.Error("AZURE_STORAGE_CONTAINER not defined for Azure plan storage")
			return nil, fmt.Errorf("AZURE_STORAGE_CONTAINER is not defined")
		}
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			slog.Error("Failed to create Azure credential",
				"error", err)
			return nil, fmt.Errorf("failed to create Azure credential: %v", err)
		}
		client, err := azblob.NewClient(
			fmt.Sprintf("https://%s.blob.core.windows.net", os.Getenv("AZURE_STORAGE_ACCOUNT_NAME")),
			cred,
			nil,
		)
		if err != nil {
			slog.Error("Failed to create Azure blob client",
				"error", err)
			return nil, fmt.Errorf("failed to create Azure blob client: %v", err)
		}
		slog.Debug("Using Azure blob storage for plan storage",
			"container", containerName)
		planStorage = &PlanStorageAzure{
			ServiceClient: client,
			ContainerName: containerName,
			Context:       context.Background(),
		}
	default:
		slog.Warn("Unknown plan upload destination, using mock storage",
			"destination", uploadDestination)
		planStorage = &MockPlanStorage{}
	}

	return planStorage, nil
}
