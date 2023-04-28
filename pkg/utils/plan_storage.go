package utils

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/google/go-github/v51/github"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type PlanStorage interface {
	StorePlan(planFileName string) error
	RetrievePlan(planFileName string) (*string, error)
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

func (psg *PlanStorageGcp) StorePlan(planFileName string) error {
	file, err := os.Open(planFileName)
	if err != nil {
		return fmt.Errorf("unable to open file: %v", err)
	}
	defer file.Close()

	obj := psg.Bucket.Object(planFileName)
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

func (psg *PlanStorageGcp) RetrievePlan(planFileName string) (*string, error) {
	obj := psg.Bucket.Object(planFileName)
	rc, err := obj.NewReader(psg.Context)
	if err != nil {
		return nil, fmt.Errorf("unable to read data from bucket: %v", err)
	}
	defer rc.Close()

	file, err := os.Create(planFileName)
	if err != nil {
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, rc); err != nil {
		return nil, fmt.Errorf("unable to write data to file: %v", err)
	}

	return &planFileName, nil
}

func (gps *GithubPlanStorage) StorePlan(planFileName string) error {
	_ = fmt.Sprintf("Skipping storing plan %s. It should be achieved using actions/upload-artifact@v3", planFileName)
	return nil
}

func (gps *GithubPlanStorage) RetrievePlan(planFileName string) (*string, error) {
	plansFilename, err := gps.DownloadLatestPlans()

	if err != nil {
		return nil, fmt.Errorf("error downloading plan: %v", err)
	}

	if plansFilename == "" {
		return nil, fmt.Errorf("no plans found for this PR")
	}

	plansFilename, err = gps.ZipManager.GetFileFromZip(plansFilename, planFileName)

	if err != nil {
		return nil, fmt.Errorf("error extracting plan: %v", err)
	}
	return &plansFilename, nil
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
