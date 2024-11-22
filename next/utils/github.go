package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/diggerhq/digger/libs/ci"
	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/google/go-github/v61/github"
	"log"
	net "net/http"
	"os"
	"strings"
	"time"
)

func createTempDir() string {
	tempDir, err := os.MkdirTemp("", "repo")
	if err != nil {
		log.Fatal(err)
	}
	return tempDir
}

func GetGithubHostname() string {
	githubHostname := os.Getenv("DIGGER_GITHUB_HOSTNAME")
	if githubHostname == "" {
		githubHostname = "github.com"
	}
	return githubHostname
}

// just a wrapper around github client to be able to use mocks
type DiggerGithubRealClientProvider struct {
}

type DiggerGithubClientMockProvider struct {
	MockedHTTPClient *net.Client
}

type GithubClientProvider interface {
	NewClient(netClient *net.Client) (*github.Client, error)
	Get(githubAppId int64, installationId int64) (*github.Client, *string, error)
	FetchCredentials(githubAppId string) (string, string, string, string, error)
}

func (gh DiggerGithubRealClientProvider) NewClient(netClient *net.Client) (*github.Client, error) {
	ghClient := github.NewClient(netClient)
	return ghClient, nil
}

func (gh DiggerGithubRealClientProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
	githubAppPrivateKey := ""
	githubAppPrivateKeyB64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")
	if githubAppPrivateKeyB64 != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(githubAppPrivateKeyB64)
		if err != nil {
			return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
		}
		githubAppPrivateKey = string(decodedBytes)
	} else {
		githubAppPrivateKey = os.Getenv("GITHUB_APP_PRIVATE_KEY")
		if githubAppPrivateKey != "" {
			log.Printf("WARNING: GITHUB_APP_PRIVATE_KEY will be deprecated in future releases, " +
				"please use GITHUB_APP_PRIVATE_KEY_BASE64 instead")
		} else {
			return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
		}
	}

	tr := net.DefaultTransport
	itr, err := ghinstallation.New(tr, githubAppId, installationId, []byte(githubAppPrivateKey))
	if err != nil {
		return nil, nil, fmt.Errorf("error initialising github app installation: %v\n", err)
	}

	token, err := itr.Token(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("error initialising git app token: %v\n", err)
	}
	ghClient, err := gh.NewClient(&net.Client{Transport: itr})
	if err != nil {
		log.Printf("error creating new client: %v", err)
	}
	return ghClient, &token, nil
}

func (gh DiggerGithubRealClientProvider) FetchCredentials(githubAppId string) (string, string, string, string, error) {
	return "", "", "", "", nil
}
func (gh DiggerGithubClientMockProvider) NewClient(netClient *net.Client) (*github.Client, error) {
	ghClient := github.NewClient(gh.MockedHTTPClient)
	return ghClient, nil
}

func (gh DiggerGithubClientMockProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
	ghClient, _ := gh.NewClient(gh.MockedHTTPClient)
	token := "token"
	return ghClient, &token, nil
}

func (gh DiggerGithubClientMockProvider) FetchCredentials(githubAppId string) (string, string, string, string, error) {
	return "", "", "", "", nil
}

func GetGithubClient(gh GithubClientProvider, installationId int64, repoFullName string) (*github.Client, *string, error) {
	installation, err := dbmodels.DB.GetGithubAppInstallationByIdAndRepo(installationId, repoFullName)
	if err != nil {
		log.Printf("Error getting installation: %v", err)
		return nil, nil, fmt.Errorf("Error getting installation: %v", err)
	}

	ghClient, token, err := gh.Get(installation.GithubAppID, installation.GithubInstallationID)
	return ghClient, token, err
}
func GetGithubService(gh GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string) (*github2.GithubService, *string, error) {
	ghClient, token, err := GetGithubClient(gh, installationId, repoFullName)
	if err != nil {
		log.Printf("Error creating github app client: %v", err)
		return nil, nil, fmt.Errorf("Error creating github app client: %v", err)
	}

	ghService := github2.GithubService{
		Client:   ghClient,
		RepoName: repoName,
		Owner:    repoOwner,
	}

	return &ghService, token, nil
}

func SetPRStatusForJobs(prService ci.PullRequestService, prNumber int, jobs []scheduler.Job) error {
	for _, job := range jobs {
		for _, command := range job.Commands {
			var err error
			switch command {
			case "digger plan":
				err = prService.SetStatus(prNumber, "pending", job.ProjectName+"/plan")
			case "digger apply":
				err = prService.SetStatus(prNumber, "pending", job.ProjectName+"/apply")
			}
			if err != nil {
				log.Printf("Error setting status: %v", err)
				return fmt.Errorf("Error setting pr status: %v", err)
			}
		}
	}
	// Report aggregate status for digger/plan or digger/apply
	if len(jobs) > 0 {
		var err error
		if scheduler.IsPlanJobs(jobs) {
			err = prService.SetStatus(prNumber, "pending", "digger/plan")
		} else {
			err = prService.SetStatus(prNumber, "pending", "digger/apply")
		}
		if err != nil {
			log.Printf("error setting status: %v", err)
			return fmt.Errorf("error setting pr status: %v", err)
		}

	} else {
		err := prService.SetStatus(prNumber, "success", "digger/plan")
		if err != nil {
			log.Printf("error setting status: %v", err)
			return fmt.Errorf("error setting pr status: %v", err)
		}
		err = prService.SetStatus(prNumber, "success", "digger/apply")
		if err != nil {
			log.Printf("error setting status: %v", err)
			return fmt.Errorf("error setting pr status: %v", err)
		}
	}

	return nil
}

func GetWorkflowIdAndUrlFromDiggerJobId(client *github.Client, repoOwner string, repoName string, diggerJobID string) (int64, string, error) {
	timeFilter := time.Now().Add(-5 * time.Minute)
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(context.Background(), repoOwner, repoName, &github.ListWorkflowRunsOptions{
		Created: ">=" + timeFilter.Format(time.RFC3339),
	})
	if err != nil {
		return 0, "#", fmt.Errorf("error listing workflow runs %v", err)
	}

	for _, workflowRun := range runs.WorkflowRuns {
		println(*workflowRun.ID)
		workflowjobs, _, err := client.Actions.ListWorkflowJobs(context.Background(), repoOwner, repoName, *workflowRun.ID, nil)
		if err != nil {
			return 0, "#", fmt.Errorf("error listing workflow jobs for run %v %v", workflowRun.ID, err)
		}

		for _, workflowjob := range workflowjobs.Jobs {
			for _, step := range workflowjob.Steps {
				if strings.Contains(*step.Name, diggerJobID) {
					return *workflowRun.ID, fmt.Sprintf("https://github.com/%v/%v/actions/runs/%v", repoOwner, repoName, *workflowRun.ID), nil
				}
			}

		}
	}
	return 0, "#", fmt.Errorf("workflow not found")
}
