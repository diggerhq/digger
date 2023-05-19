package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/google/go-github/v51/github"
)

type PlanStorage interface {
	StorePlan(localPlanFilePath string, storedPlanFilePath string) error
	RetrievePlan(localPlanFilePath string, storedPlanFilePath string) (*string, error)
	DeleteStoredPlan(storedPlanFilePath string) error
	PlanExists(storedPlanFilePath string) (bool, error)
}

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
	ZipManager        Zipper
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

func (gps *GithubPlanStorage) RetrievePlan(localPlanFilePath string, storedPlanFilePath string) (*string, error) {
	plansFilename, err := gps.DownloadLatestPlans()

	if err != nil {
		return nil, fmt.Errorf("error downloading plan: %v", err)
	}

	if plansFilename == "" {
		return nil, fmt.Errorf("no plans found for this PR")
	}

	plansFilename, err = gps.ZipManager.GetFileFromZip(plansFilename, storedPlanFilePath)

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

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, "plans-"+strconv.Itoa(gps.PullRequestNumber))

	if latestPlans == nil {
		return false, nil
	}
	return true, nil
}

func (gps *GithubPlanStorage) DeleteStoredPlan(storedPlanFilePath string) error {
	return nil
}

func (gps *GithubPlanStorage) DownloadLatestPlans() (string, error) {
	artifacts, _, err := gps.Client.Actions.ListArtifacts(context.Background(), gps.Owner, gps.RepoName, &github.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		return "", err
	}

	latestPlans := getLatestArtifactWithName(artifacts.Artifacts, "plans-"+strconv.Itoa(gps.PullRequestNumber))

	if latestPlans == nil {
		return "", nil
	}

	downloadUrl, _, err := gps.Client.Actions.DownloadArtifact(context.Background(), gps.Owner, gps.RepoName, *latestPlans.ID, true)

	if err != nil {
		return "", err
	}
	filename := "plans-" + strconv.Itoa(gps.PullRequestNumber) + ".zip"

	err = downloadArtifactIntoFile(gps.Client.Client(), downloadUrl, filename)

	if err != nil {
		return "", err
	}
	return filename, nil
}

func downloadArtifactIntoFile(client *http.Client, artifactUrl *url.URL, outputFile string) error {

	req, err := http.NewRequest("GET", artifactUrl.String(), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download artifact, status code: %d", resp.StatusCode)
	}

	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

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
