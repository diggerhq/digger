package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/google/go-github/v58/github"
)

type PlanStorageGcp struct {
	Client  *storage.Client
	Bucket  *storage.BucketHandle
	Context context.Context
}

type GithubPlanStorage struct {
	Client            *github.Client
	Owner             string
	RepoName          string
	PullRequestNumber int
	ZipManager        utils.Zipper
}

func (psg *PlanStorageGcp) PlanExists(storedPlanFilePath string) (bool, error) {
	obj := psg.Bucket.Object(storedPlanFilePath)
	_, err := obj.Attrs(psg.Context)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, fmt.Errorf("unable to get object attributes: %v", err)
	}
	return true, nil
}

func (psg *PlanStorageGcp) StorePlan(localPlanFilePath string, storedPlanFilePath string) error {
	file, err := os.Open(localPlanFilePath)
	if err != nil {
		return fmt.Errorf("unable to open file: %v", err)
	}
	defer file.Close()

	obj := psg.Bucket.Object(storedPlanFilePath)
	wc := obj.NewWriter(psg.Context)

	if _, err = io.Copy(wc, file); err != nil {
		wc.Close()
		return fmt.Errorf("unable to write data to bucket: %v", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("unable to close writer: %v", err)
	}

	return nil
}

func (psg *PlanStorageGcp) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
	// TODO: implement me
	return nil
}

func (psg *PlanStorageGcp) RetrievePlan(localPlanFilePath string, storedPlanFilePath string) (*string, error) {
	obj := psg.Bucket.Object(storedPlanFilePath)
	rc, err := obj.NewReader(psg.Context)
	if err != nil {
		return nil, fmt.Errorf("unable to read data from bucket: %v", err)
	}
	defer rc.Close()

	file, err := os.Create(localPlanFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, rc); err != nil {
		return nil, fmt.Errorf("unable to write data to file: %v", err)
	}
	fileName, err := filepath.Abs(file.Name())
	if err != nil {
		return nil, fmt.Errorf("unable to get absolute path for file: %v", err)
	}
	return &fileName, nil
}

func (psg *PlanStorageGcp) DeleteStoredPlan(storedPlanFilePath string) error {
	obj := psg.Bucket.Object(storedPlanFilePath)
	err := obj.Delete(psg.Context)

	if err != nil {
		return fmt.Errorf("unable to delete file '%v' from bucket: %v", storedPlanFilePath, err)
	}
	return nil
}

func (gps *GithubPlanStorage) StorePlan(localPlanFilePath string, storedPlanFilePath string) error {
	_ = fmt.Sprintf("Skipping storing plan %s. It should be achieved using actions/upload-artifact@v3", localPlanFilePath)
	return nil
}

func (gps *GithubPlanStorage) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
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
	createArtifactResponse, err := doRequest("POST", createArtifactURL, headers, createArtifactBody)
	if createArtifactResponse == nil || err != nil {
		return fmt.Errorf("Could not create artifact with github %v", err)
	}
	defer createArtifactResponse.Body.Close()

	// Extract Resource URL
	createArtifactResponseBody, _ := io.ReadAll(createArtifactResponse.Body)
	var createArtifactResponseMap map[string]interface{}
	json.Unmarshal(createArtifactResponseBody, &createArtifactResponseMap)
	resourceURL := createArtifactResponseMap["fileContainerResourceUrl"].(string)

	// Upload Data
	uploadURL := fmt.Sprintf("%s?itemPath=%s/%s", resourceURL, artifactName, fileName)
	uploadData := fileContents
	dataLen := len(uploadData)
	headers["Content-Type"] = "application/octet-stream"
	headers["Content-Range"] = fmt.Sprintf("bytes 0-%v/%v", dataLen-1, dataLen)
	_, err = doRequest("PUT", uploadURL, headers, uploadData)
	if err != nil {
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
	_, err = doRequest("PATCH", updateArtifactURL, headers, updateArtifactBody)
	if err != nil {
		return fmt.Errorf("could finalize artefact upload: %v", err)
	}

	return nil
}

func doRequest(method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	fmt.Printf("Sending request %v %v\n", method, url)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, fmt.Errorf("Error creating request:", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, fmt.Errorf("Error creating request:", err)
	}
	if resp.StatusCode >= 400 {
		fmt.Printf("url: %v", url)
		fmt.Println("Request failed with status code:", resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("body: %v", string(body))
		return nil, fmt.Errorf("error creating request:", err)
	}
	return resp, nil
}

func (gps *GithubPlanStorage) RetrievePlan(localPlanFilePath string, storedPlanFilePath string) (*string, error) {
	plansFilename, err := gps.DownloadLatestPlans(storedPlanFilePath)

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

func (gps *GithubPlanStorage) PlanExists(storedPlanFilePath string) (bool, error) {
	artifacts, _, err := gps.Client.Actions.ListArtifacts(context.Background(), gps.Owner, gps.RepoName, &github.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		return false, err
	}

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, storedPlanFilePath)

	if latestPlans == nil {
		return false, nil
	}
	return true, nil
}

func (gps *GithubPlanStorage) DeleteStoredPlan(storedPlanFilePath string) error {
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

	log.Printf("Download url received: %v", downloadUrl)
	err = downloadArtifactIntoFile(downloadUrl, filename)

	if err != nil {
		return "", err
	}
	return filename, nil
}

func downloadArtifactIntoFile(artifactUrl *url.URL, outputFile string) error {

	cmd := exec.Command("wget", "-O", outputFile, artifactUrl.String())
	stdout, err := cmd.Output()
	if err != nil {
		return err
	}

	fmt.Printf("output of wget command: %v", string(stdout))

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
