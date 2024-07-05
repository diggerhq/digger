package main

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/bitbucket"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
	orchestrator_github "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/policy"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"os"
	"time"
)

type RunConfig struct {
	RepoNamespace  string `mapstructure:"repo-namespace"`
	Reporter       string `mapstructure:"reporter"`
	PRNumber       int    `mapstructure:"pr-number"`
	CommentID      string `mapstructure:"comment-id"`
	Actor          string `mapstructure:"actor"`
	GithubToken    string `mapstructure:"github-token"`
	BitbucketToken string `mapstructure:"bitbucket-token"`
}

func (r *RunConfig) GetServices() (*ci.PullRequestService, *ci.OrgService, *reporting.Reporter, error) {
	var prService ci.PullRequestService
	var orgService ci.OrgService
	var reporter reporting.Reporter
	switch r.Reporter {
	case "github":
		repoOwner, repositoryName := utils.ParseRepoNamespace(r.RepoNamespace)
		prService, _ = orchestrator_github.GithubServiceProviderBasic{}.NewService(r.GithubToken, repositoryName, repoOwner)
		orgService, _ = orchestrator_github.GithubServiceProviderBasic{}.NewService(r.GithubToken, r.RepoNamespace, r.Actor)
		reporter = &reporting.CiReporter{
			CiService:         prService,
			ReportStrategy:    ReportStrategy,
			PrNumber:          r.PRNumber,
			IsSupportMarkdown: true,
		}
	case "bitbucket":
		repoOwner, repositoryName := utils.ParseRepoNamespace(r.RepoNamespace)
		prService = bitbucket.BitbucketAPI{
			AuthToken:     r.BitbucketToken,
			HttpClient:    http.Client{},
			RepoWorkspace: repoOwner,
			RepoName:      repositoryName,
		}
		orgService = bitbucket.BitbucketAPI{
			AuthToken:     "",
			HttpClient:    http.Client{},
			RepoWorkspace: repoOwner,
			RepoName:      repositoryName,
		}
		reporter = &reporting.CiReporter{
			CiService:         prService,
			ReportStrategy:    ReportStrategy,
			PrNumber:          r.PRNumber,
			IsSupportMarkdown: false,
		}

	case "stdout":
		print("Using Stdout.")
		reporter = &reporting.StdOutReporter{}
		prService = orchestrator_github.MockCiService{}
		orgService = orchestrator_github.MockCiService{}
	default:
		return nil, nil, nil, fmt.Errorf("unknown reporter: %v", r.Reporter)

	}

	return &prService, &orgService, &reporter, nil
}

var BackendApi backendapi.Api
var ReportStrategy reporting.ReportStrategy
var lock locking.Lock

func PreRun(cmd *cobra.Command, args []string) {

	hostName := os.Getenv("DIGGER_HOSTNAME")
	token := os.Getenv("DIGGER_TOKEN")
	BackendApi = NewBackendApi(hostName, token)

	if os.Getenv("REPORTING_STRATEGY") == "comments_per_run" || os.Getenv("ACCUMULATE_PLANS") == "true" {
		ReportStrategy = &reporting.CommentPerRunStrategy{
			TimeOfRun: time.Now(),
		}
	} else if os.Getenv("REPORTING_STRATEGY") == "latest_run_comment" {
		ReportStrategy = &reporting.LatestRunCommentStrategy{
			TimeOfRun: time.Now(),
		}
	} else {
		ReportStrategy = &reporting.MultipleCommentsStrategy{}
	}

	var err error
	if os.Getenv("NO_BACKEND") == "true" {
		lock, err = locking.GetLock()
	} else {
		log.Printf("Warning: not performing locking in cli since digger is invoked with orchestrator mode, any arguments to LOCKING_PROVIDER will be ignored")
		lock = locking.NoOpLock{}
		err = nil
	}
	if err != nil {
		log.Printf("Failed to create lock provider. %s\n", err)
		os.Exit(2)
	}
	log.Println("Lock provider has been created successfully")
}

func NewBackendApi(hostName string, authToken string) backendapi.Api {
	var backendApi backendapi.Api
	if os.Getenv("NO_BACKEND") == "true" {
		log.Println("WARNING: running in 'backendless' mode. Features that require backend will not be available.")
		backendApi = backendapi.NoopApi{}
	} else {
		backendApi = backendapi.DiggerApi{
			DiggerHost: hostName,
			AuthToken:  authToken,
			HttpClient: http.DefaultClient,
		}
	}
	return backendApi
}

func NewPolicyChecker(hostname string, organisationName string, authToken string) policy.Checker {
	var policyChecker policy.Checker
	if os.Getenv("NO_BACKEND") == "true" {
		log.Println("WARNING: running in 'backendless' mode. Features that require backend will not be available.")
		policyChecker = policy.NoOpPolicyChecker{}
	} else {
		policyChecker = policy.DiggerPolicyChecker{
			PolicyProvider: &policy.DiggerHttpPolicyProvider{
				DiggerHost:         hostname,
				DiggerOrganisation: organisationName,
				AuthToken:          authToken,
				HttpClient:         http.DefaultClient,
			}}
	}
	return policyChecker
}

var rootCmd = &cobra.Command{
	Use:              "digger",
	Short:            "An open source IaC orchestration tool",
	PersistentPreRun: PreRun,
}
