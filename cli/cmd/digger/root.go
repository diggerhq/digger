package main

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/bitbucket"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
	orchestrator_github "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	locking2 "github.com/diggerhq/digger/libs/locking"
	core_policy "github.com/diggerhq/digger/libs/policy"
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

var PolicyChecker core_policy.Checker
var BackendApi backendapi.Api
var ReportStrategy reporting.ReportStrategy
var lock locking2.Lock

func PreRun(cmd *cobra.Command, args []string) {
	if cmd.Name() == "run_spec" {
		return
	}

	hostName := os.Getenv("DIGGER_HOSTNAME")
	token := os.Getenv("DIGGER_TOKEN")
	//orgName := os.Getenv("DIGGER_ORGANISATION")
	BackendApi = backendapi.NewBackendApi(hostName, token)
	//PolicyChecker = policy.NewPolicyChecker(hostName, orgName, token)

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
		lock, err = locking2.GetLock()
	} else {
		log.Printf("Warning: not performing locking in cli since digger is invoked with orchestrator mode, any arguments to LOCKING_PROVIDER will be ignored")
		lock = locking2.NoOpLock{}
		err = nil
	}
	if err != nil {
		log.Printf("Failed to create lock provider. %s\n", err)
		os.Exit(2)
	}
	log.Println("Lock provider has been created successfully")
}

var rootCmd = &cobra.Command{
	Use:              "digger",
	Short:            "An open source IaC orchestration tool",
	PersistentPreRun: PreRun,
}
