package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	net "net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/ci"
	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
)

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
	slog.Debug("Getting GitHub client",
		"githubAppId", githubAppId,
		"installationId", installationId,
	)

	githubAppPrivateKey := ""
	githubAppPrivateKeyB64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")
	if githubAppPrivateKeyB64 != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(githubAppPrivateKeyB64)
		if err != nil {
			slog.Error("Failed to decode GITHUB_APP_PRIVATE_KEY_BASE64", "error", err)
			return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
		}
		githubAppPrivateKey = string(decodedBytes)
	} else {
		githubAppPrivateKey = os.Getenv("GITHUB_APP_PRIVATE_KEY")
		if githubAppPrivateKey != "" {
			slog.Warn("GITHUB_APP_PRIVATE_KEY will be deprecated in future releases, please use GITHUB_APP_PRIVATE_KEY_BASE64 instead")
		} else {
			slog.Error("Missing GitHub app private key", "required", "GITHUB_APP_PRIVATE_KEY_BASE64")
			return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
		}
	}

	tr := net.DefaultTransport
	itr, err := ghinstallation.New(tr, githubAppId, installationId, []byte(githubAppPrivateKey))
	if err != nil {
		slog.Error("Failed to initialize GitHub app installation",
			"githubAppId", githubAppId,
			"installationId", installationId,
			"error", err,
		)
		return nil, nil, fmt.Errorf("error initialising github app installation: %v\n", err)
	}

	token, err := itr.Token(context.Background())
	if err != nil {
		slog.Error("Failed to get GitHub app token",
			"githubAppId", githubAppId,
			"installationId", installationId,
			"error", err,
		)
		return nil, nil, fmt.Errorf("error initialising git app token: %v\n", err)
	}

	clientWithLogging := &net.Client{
		Transport: &LoggingRoundTripper{Rt: itr},
	}

	ghClient, err := gh.NewClient(clientWithLogging)
	if err != nil {
		slog.Error("Failed to create GitHub client", "error", err)
		return nil, nil, fmt.Errorf("error creating new client: %v", err)
	}

	slog.Debug("Successfully obtained GitHub client and token",
		"githubAppId", githubAppId,
		"installationId", installationId,
	)

	return ghClient, &token, nil
}

func (gh DiggerGithubRealClientProvider) FetchCredentials(githubAppId string) (string, string, string, string, error) {
	clientId := os.Getenv("GITHUB_APP_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_APP_CLIENT_SECRET")
	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	privateKeyb64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")
	return clientId, clientSecret, webhookSecret, privateKeyb64, nil
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
	return "clientId", "clientSecret", "", "", nil
}

func GetGithubClient(gh GithubClientProvider, installationId int64, repoFullName string) (*github.Client, *string, error) {
	installation, err := models.DB.GetGithubAppInstallationByIdAndRepo(installationId, repoFullName)
	if err != nil {
		slog.Error("Failed to get GitHub installation",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return nil, nil, fmt.Errorf("Error getting installation: %v", err)
	}

	ghClient, token, err := gh.Get(installation.GithubAppId, installation.GithubInstallationId)
	return ghClient, token, err
}

func GetGithubClientFromAppId(gh GithubClientProvider, installationId int64, githubAppId int64, repoFullName string) (*github.Client, *string, error) {
	ghClient, token, err := gh.Get(githubAppId, installationId)
	return ghClient, token, err
}

func GetGithubService(gh GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string) (*github2.GithubService, *string, error) {
	ghClient, token, err := GetGithubClient(gh, installationId, repoFullName)
	if err != nil {
		slog.Error("Failed to create GitHub client",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return nil, nil, fmt.Errorf("Error creating github app client: %v", err)
	}

	ghService := github2.GithubService{
		Client:   ghClient,
		RepoName: repoName,
		Owner:    repoOwner,
	}

	slog.Debug("Created GitHub service",
		"owner", repoOwner,
		"repoName", repoName,
	)

	return &ghService, token, nil
}

func SetPRCommitStatusForJobs(prService ci.PullRequestService, prNumber int, jobs []scheduler.Job) error {
	slog.Info("Setting PR status for jobs",
		"prNumber", prNumber,
		"jobCount", len(jobs),
	)

	for _, job := range jobs {
		for _, command := range job.Commands {
			var err error
			switch command {
			case "digger plan":
				slog.Debug("Setting PR status for plan",
					"prNumber", prNumber,
					"project", job.ProjectName,
				)
				err = prService.SetStatus(prNumber, "pending", job.GetProjectAlias()+"/plan")
			case "digger apply":
				slog.Debug("Setting PR status for apply",
					"prNumber", prNumber,
					"project", job.ProjectName,
				)
				err = prService.SetStatus(prNumber, "pending", job.GetProjectAlias()+"/apply")
			}
			if err != nil {
				slog.Error("Failed to set PR status",
					"prNumber", prNumber,
					"project", job.ProjectName,
					"command", command,
					"error", err,
				)
				return fmt.Errorf("Error setting pr status: %v", err)
			}
		}
	}

	// Report aggregate status for digger/plan or digger/apply
	if len(jobs) > 0 {
		var err error
		if scheduler.IsPlanJobs(jobs) {
			slog.Debug("Setting aggregate plan status", "prNumber", prNumber)
			err = prService.SetStatus(prNumber, "pending", "digger/plan")
		} else {
			slog.Debug("Setting aggregate apply status", "prNumber", prNumber)
			err = prService.SetStatus(prNumber, "pending", "digger/apply")
		}
		if err != nil {
			slog.Error("Failed to set aggregate PR status",
				"prNumber", prNumber,
				"error", err,
			)
			return fmt.Errorf("error setting pr status: %v", err)
		}
	} else {
		slog.Debug("Setting success status for empty job list", "prNumber", prNumber)

		err := prService.SetStatus(prNumber, "success", "digger/plan")
		if err != nil {
			slog.Error("Failed to set success plan status", "prNumber", prNumber, "error", err)
			return fmt.Errorf("error setting pr status: %v", err)
		}

		err = prService.SetStatus(prNumber, "success", "digger/apply")
		if err != nil {
			slog.Error("Failed to set success apply status", "prNumber", prNumber, "error", err)
			return fmt.Errorf("error setting pr status: %v", err)
		}
	}

	slog.Info("Successfully set PR status", "prNumber", prNumber)
	return nil
}

// Checks are the more modern github way as opposed to "commit status"
// With checks you also get to set a page representing content of the check
func SetPRCheckForJobs(ghService *github2.GithubService, prNumber int, jobs []scheduler.Job, commitSha string) (*CheckRunData, map[string]CheckRunData, error) {
	slog.Info("commitSha", "commitsha", commitSha)
	slog.Info("Setting PR status for jobs",
		"prNumber", prNumber,
		"jobCount", len(jobs),
		"commitSha", commitSha,
	)
	var batchCheckRunId CheckRunData
	var jobCheckRunIds = make(map[string]CheckRunData)

	for _, job := range jobs {
		for _, command := range job.Commands {
			var cr *github.CheckRun
			var err error
			switch command {
			case "digger plan":
				slog.Debug("Setting PR status for plan",
					"prNumber", prNumber,
					"project", job.ProjectName,
				)
				var actions []*github.CheckRunAction
				cr, err = ghService.CreateCheckRun(job.GetProjectAlias()+"/plan", "in_progress", "", "Waiting for plan...", "", "Plan result will appear here", commitSha, actions)
				jobCheckRunIds[job.ProjectName] = CheckRunData{
						Id: strconv.FormatInt(*cr.ID, 10),
						Url: *cr.HTMLURL,
					}

			case "digger apply":
				slog.Debug("Setting PR status for apply",
					"prNumber", prNumber,
					"project", job.ProjectName,
				)
				cr, err = ghService.CreateCheckRun(job.GetProjectAlias()+"/apply", "in_progress", "", "Waiting for apply...", "", "Apply result will appear here", commitSha, nil)
				jobCheckRunIds[job.ProjectName] = CheckRunData{
					Id: strconv.FormatInt(*cr.ID, 10),
					Url: *cr.URL,
				}
			}
			if err != nil {
				slog.Error("Failed to set job PR status",
					"prNumber", prNumber,
					"project", job.ProjectName,
					"command", command,
					"error", err,
				)
				return nil, nil, fmt.Errorf("Error setting pr status: %v", err)
			}
		}
	}

	// Report aggregate status for digger/plan or digger/apply
	jobsSummaryTable := GetInitialJobSummary(jobs)
	if len(jobs) > 0 {
		var err error
		var cr *github.CheckRun
		if scheduler.IsPlanJobs(jobs) {
			slog.Debug("Setting aggregate plan status", "prNumber", prNumber)
			cr, err = ghService.CreateCheckRun("digger/plan", "in_progress", "", "Pending start...", "", jobsSummaryTable, commitSha, nil)
			batchCheckRunId = CheckRunData{
				Id: strconv.FormatInt(*cr.ID, 10),
				Url: *cr.HTMLURL,
			}
		} else {
			slog.Debug("Setting aggregate apply status", "prNumber", prNumber)
			cr, err = ghService.CreateCheckRun("digger/apply", "in_progress", "", "Pending start...", "", jobsSummaryTable, commitSha, nil)
			batchCheckRunId = CheckRunData{
				Id: strconv.FormatInt(*cr.ID, 10),
				Url: *cr.HTMLURL,
			}
		}
		if err != nil {
			slog.Error("Failed to set aggregate PR status",
				"prNumber", prNumber,
				"error", err,
			)
			return nil, nil, fmt.Errorf("error setting pr status: %v", err)
		}
	} else {
		slog.Debug("Setting success status for empty job list", "prNumber", prNumber)
		_, err := ghService.CreateCheckRun("digger/plan", "completed", "success", "No impacted projects", "Check your configuration and files changed if this is unexpected", "digger/plan", commitSha, nil)
		if err != nil {
			slog.Error("Failed to set success plan status", "prNumber", prNumber, "error", err)
			return nil, nil, fmt.Errorf("error setting pr status: %v", err)
		}

		_, err = ghService.CreateCheckRun("digger/apply", "completed", "success", "No impacted projects", "Check your configuration and files changed if this is unexpected", "digger/apply", commitSha, nil)
		if err != nil {
			slog.Error("Failed to set success apply status", "prNumber", prNumber, "error", err)
			return nil, nil, fmt.Errorf("error setting pr status: %v", err)
		}
	}

	slog.Info("Successfully set PR status", "prNumber", prNumber)
	return &batchCheckRunId, jobCheckRunIds, nil
}

func GetActionsForBatch(batch *models.DiggerBatch) []*github.CheckRunAction {
	batchActions := make([]*github.CheckRunAction, 0)
	if batch.Status == scheduler.BatchJobSucceeded {
		batchActions = append(batchActions, &github.CheckRunAction{
			Label:       "Apply all", // max 20 chars
			Description: "Apply all jobs", // max 40 chars
			Identifier:  batch.DiggerBatchID, // max 20 chars
		})
	}
	return batchActions
}

func GetActionsForJob(job *models.DiggerJob) []*github.CheckRunAction {
	batchActions := make([]*github.CheckRunAction, 0)
	if job.Status == scheduler.DiggerJobSucceeded {
		batchActions = append(batchActions, &github.CheckRunAction{
			Label:       "Apply job", // max 20 chars
			Description: "Apply this job", // max 40 chars
			Identifier:  job.DiggerJobID, // max 20 chars
		})
	}
	return batchActions
}

func GetGithubHostname() string {
	githubHostname := os.Getenv("DIGGER_GITHUB_HOSTNAME")
	if githubHostname == "" {
		githubHostname = "github.com"
	}
	return githubHostname
}

func GetWorkflowIdAndUrlFromDiggerJobId(client *github.Client, repoOwner string, repoName string, diggerJobID string) (int64, string, error) {
	slog.Debug("Looking for workflow for job",
		"diggerJobId", diggerJobID,
		slog.Group("repository",
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
	)

	timeFilter := time.Now().Add(-5 * time.Minute)
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(context.Background(), repoOwner, repoName, &github.ListWorkflowRunsOptions{
		Created: ">=" + timeFilter.Format(time.RFC3339),
	})
	if err != nil {
		slog.Error("Failed to list workflow runs",
			"repoOwner", repoOwner,
			"repoName", repoName,
			"error", err,
		)
		return 0, "#", fmt.Errorf("error listing workflow runs %v", err)
	}

	slog.Debug("Searching through workflow runs",
		"count", len(runs.WorkflowRuns),
		"timeFilter", timeFilter.Format(time.RFC3339),
	)

	for _, workflowRun := range runs.WorkflowRuns {
		workflowjobs, _, err := client.Actions.ListWorkflowJobs(context.Background(), repoOwner, repoName, *workflowRun.ID, nil)
		if err != nil {
			slog.Error("Failed to list workflow jobs",
				"workflowRunId", *workflowRun.ID,
				"error", err,
			)
			return 0, "#", fmt.Errorf("error listing workflow jobs for run %v %v", workflowRun.ID, err)
		}

		for _, workflowjob := range workflowjobs.Jobs {
			for _, step := range workflowjob.Steps {
				if strings.Contains(*step.Name, diggerJobID) {
					workflowUrl := fmt.Sprintf("https://%v/%v/%v/actions/runs/%v", GetGithubHostname(), repoOwner, repoName, *workflowRun.ID)

					slog.Info("Found workflow for job",
						"diggerJobId", diggerJobID,
						"workflowRunId", *workflowRun.ID,
						"workflowUrl", workflowUrl,
					)

					return *workflowRun.ID, workflowUrl, nil
				}
			}
		}
	}

	slog.Warn("No workflow found for job",
		"diggerJobId", diggerJobID,
		"repoOwner", repoOwner,
		"repoName", repoName,
	)

	return 0, "#", fmt.Errorf("workflow not found")
}
