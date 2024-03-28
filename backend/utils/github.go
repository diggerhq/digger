package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	net "net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/orchestrator"
	github2 "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v58/github"
)

func createTempDir() string {
	tempDir, err := os.MkdirTemp("", "repo")
	if err != nil {
		log.Fatal(err)
	}
	return tempDir
}

type action func(string) error

func CloneGitRepoAndDoAction(repoUrl string, branch string, token string, action action) error {
	dir := createTempDir()
	cloneOptions := git.CloneOptions{
		URL:           repoUrl,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Depth:         1,
		SingleBranch:  true,
	}

	if token != "" {
		cloneOptions.Auth = &http.BasicAuth{
			Username: "x-access-token", // anything except an empty string
			Password: token,
		}
	}

	_, err := git.PlainClone(dir, false, &cloneOptions)
	if err != nil {
		log.Printf("PlainClone error: %v\n", err)
		return err
	}
	defer os.RemoveAll(dir)

	err = action(dir)
	if err != nil {
		log.Printf("error performing action: %v", err)
		return err
	}

	return nil

}

// just a wrapper around github client to be able to use mocks
type DiggerGithubRealClientProvider struct {
}

type DiggerGithubClientMockProvider struct {
	MockedHTTPClient *net.Client
}

type GithubClientProvider interface {
	Get(githubAppId int64, installationId int64) (*github.Client, *string, error)
}

func (gh *DiggerGithubRealClientProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
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
	ghClient := github.NewClient(&net.Client{Transport: itr})
	return ghClient, &token, nil
}

func (gh *DiggerGithubClientMockProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
	ghClient := github.NewClient(gh.MockedHTTPClient)
	token := "token"
	return ghClient, &token, nil
}

func GetGithubClient(gh GithubClientProvider, installationId int64, repoFullName string) (*github.Client, *string, error) {
	installation, err := models.DB.GetGithubAppInstallationByIdAndRepo(installationId, repoFullName)
	if err != nil {
		log.Printf("Error getting installation: %v", err)
		return nil, nil, fmt.Errorf("Error getting installation: %v", err)
	}

	_, err = models.DB.GetGithubApp(installation.GithubAppId)
	if err != nil {
		log.Printf("Error getting app: %v", err)
		return nil, nil, fmt.Errorf("Error getting app: %v", err)
	}

	ghClient, token, err := gh.Get(installation.GithubAppId, installation.GithubInstallationId)
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

func SetPRStatusForJobs(prService *github2.GithubService, prNumber int, jobs []orchestrator.Job) error {
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
				log.Printf("Erorr setting status: %v", err)
				return fmt.Errorf("Error setting pr status: %v", err)
			}
		}
	}
	// Report aggregate status for digger/plan or digger/apply
	if len(jobs) > 0 {
		var err error
		if orchestrator.IsPlanJobs(jobs) {
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
		Created: ">=" + timeFilter.Format("2006-01-02T15:04:05"),
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

func TriggerGithubWorkflow(client *github.Client, repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error {
	log.Printf("TriggerGithubWorkflow: repoOwner: %v, repoName: %v, commentId: %v", repoOwner, repoName, commentId)
	_, err := client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), repoOwner, repoName, job.WorkflowFile, github.CreateWorkflowDispatchEventRequest{
		Ref:    job.Batch.BranchName,
		Inputs: map[string]interface{}{"job": jobString, "id": job.DiggerJobID, "comment_id": strconv.FormatInt(commentId, 10)},
	})

	return err

}
