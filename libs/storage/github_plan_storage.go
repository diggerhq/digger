package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/diggerhq/digger/libs/locking/gcp"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v61/github"
)

type GithubPlanStorage struct {
	Client            *github.Client
	Owner             string
	RepoName          string
	PullRequestNumber int
	ZipManager        Zipper
}

func (gps *GithubPlanStorage) StorePlanFile(fileContents []byte, localFilePath string, artifactName string, storedPlanFilePath string) error {
	files := []string{localFilePath}
	_, err := UploadArtifact(context.Background(), artifactName, files, "/", &UploadArtifactOptions{
		RetentionDays:    nil,
		CompressionLevel: nil,
	})
	if err != nil {
		return err
	}
	return nil
}

func doRequest(method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if resp.StatusCode >= 400 {
		fmt.Printf("url: %v", url)
		fmt.Println("Request failed with status code:", resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("body: %v", string(body))
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	return resp, nil
}

func (gps *GithubPlanStorage) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	plansFilename, err := gps.DownloadLatestPlans(artifactName)

	if err != nil {
		return nil, fmt.Errorf("error downloading plan: %v", err)
	}

	if plansFilename == "" {
		return nil, fmt.Errorf("no plans found for this PR")
	}

	plansFilename, err = gps.ZipManager.GetFileFromZip(plansFilename, localPlanFilePath)

	if err != nil {
		return nil, fmt.Errorf("error extracting plan: %v", err)
	}
	return &plansFilename, nil
}

func (gps *GithubPlanStorage) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	artifacts, _, err := gps.Client.Actions.ListArtifacts(context.Background(), gps.Owner, gps.RepoName, &github.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		return false, err
	}

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, artifactName)

	if latestPlans == nil {
		return false, nil
	}
	return true, nil
}

func (gps *GithubPlanStorage) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	return nil
}

func (gps *GithubPlanStorage) DownloadLatestPlans(storedPlanFilePath string) (string, error) {
	artifacts, _, err := gps.Client.Actions.ListArtifacts(context.Background(), gps.Owner, gps.RepoName, &github.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		return "", err
	}

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, storedPlanFilePath)

	if latestPlans == nil {
		return "", nil
	}

	downloadUrl, _, err := gps.Client.Actions.DownloadArtifact(context.Background(), gps.Owner, gps.RepoName, *latestPlans.ID, 0)

	if err != nil {
		return "", err
	}
	filename := storedPlanFilePath + ".zip"

	err = downloadArtifactIntoFile(downloadUrl, filename)

	if err != nil {
		return "", err
	}
	return filename, nil
}

func downloadArtifactIntoFile(artifactUrl *url.URL, outputFile string) error {

	cmd := exec.Command("wget", "-O", outputFile, artifactUrl.String())
	_, err := cmd.Output()
	if err != nil {
		return err
	}

	log.Printf("Successfully fetched plan artifact into %v", outputFile)

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

	return latest
}

// TODO: refactor this function to make it fully parametarised and no reliance on env variables
func NewPlanStorage(ghToken string, ghRepoOwner string, ghRepositoryName string, prNumber *int) (PlanStorage, error) {
	var planStorage PlanStorage

	uploadDestination := strings.ToLower(os.Getenv("PLAN_UPLOAD_DESTINATION"))
	switch {
	case uploadDestination == "github":
		if ghToken == "" {
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		zipManager := Zipper{}
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
			return nil, fmt.Errorf("GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET is not defined")
		}
		bucket := client.Bucket(bucketName)
		planStorage = &PlanStorageGcp{
			Client:  client,
			Bucket:  bucket,
			Context: ctx,
		}
	case uploadDestination == "aws":
		ctx, client, err := GetAWSStorageClient()
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Failed to create AWS storage client: %s", err))
		}
		bucketName := strings.ToLower(os.Getenv("AWS_S3_BUCKET"))
		if bucketName == "" {
			return nil, fmt.Errorf("AWS_S3_BUCKET is not defined")
		}
		planStorage = &PlanStorageAWS{
			Context: ctx,
			Client:  client,
			Bucket:  bucketName,
		}
	case uploadDestination == "gitlab":
	//TODO implement me
	default:
		log.Printf("unknown plan destination type %v, using noop", uploadDestination)
		planStorage = &MockPlanStorage{}
	}

	return planStorage, nil
}
