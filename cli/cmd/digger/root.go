package main

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/backend"
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	core_reporting "github.com/diggerhq/digger/cli/pkg/core/reporting"
	github_pkg "github.com/diggerhq/digger/cli/pkg/github"
	"github.com/diggerhq/digger/cli/pkg/locking"
	"github.com/diggerhq/digger/cli/pkg/policy"
	"github.com/diggerhq/digger/cli/pkg/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type RunConfig struct {
	RepoNamespace string `mapstructure:"repo-namespace"`
	Reporter      string `mapstructure:"reporter"`
	PRNumber      int    `mapstructure:"pr-number"`
	CommentID     string `mapstructure:"comment-id"`
	Actor         string `mapstructure:"actor"`
	GithubToken   string `mapstructure:"github-token"`
}

func (r *RunConfig) GetServices() (*orchestrator.PullRequestService, *orchestrator.OrgService, *core_reporting.Reporter, error) {
	var prService orchestrator.PullRequestService
	var orgService orchestrator.OrgService
	var reporter core_reporting.Reporter
	switch r.Reporter {
	case "github":
		splitRepositoryName := strings.Split(r.RepoNamespace, "/")
		repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
		prService = orchestrator_github.NewGitHubService(r.GithubToken, repositoryName, repoOwner)
		orgService = orchestrator_github.NewGitHubService(r.GithubToken, r.RepoNamespace, r.Actor)
		reporter = &reporting.CiReporter{
			CiService:         prService,
			ReportStrategy:    ReportStrategy,
			PrNumber:          r.PRNumber,
			IsSupportMarkdown: true,
		}
	case "stdout":
		print("Using Stdout.")
		reporter = &reporting.StdoutReporter{
			ReportStrategy:    ReportStrategy,
			IsSupportMarkdown: true,
		}
		prService = github_pkg.MockCiService{}
		orgService = github_pkg.MockCiService{}
	default:
		return nil, nil, nil, fmt.Errorf("unknown reporter: %v", r.Reporter)

	}

	return &prService, &orgService, &reporter, nil
}

var PolicyChecker core_policy.Checker
var BackendApi core_backend.Api
var ReportStrategy reporting.ReportStrategy
var lock core_locking.Lock

func PreRun(cmd *cobra.Command, args []string) {

	if os.Getenv("NO_BACKEND") == "true" {
		log.Println("WARNING: running in 'backendless' mode. Features that require backend will not be available.")
		PolicyChecker = policy.NoOpPolicyChecker{}
		BackendApi = backend.NoopApi{}
	} else if os.Getenv("DIGGER_TOKEN") != "" {
		if os.Getenv("DIGGER_ORGANISATION") == "" {
			log.Fatalf("Token specified but missing organisation: DIGGER_ORGANISATION. Please set this value in action digger_config.")
		}
		PolicyChecker = policy.DiggerPolicyChecker{
			PolicyProvider: &policy.DiggerHttpPolicyProvider{
				DiggerHost:         os.Getenv("DIGGER_HOSTNAME"),
				DiggerOrganisation: os.Getenv("DIGGER_ORGANISATION"),
				AuthToken:          os.Getenv("DIGGER_TOKEN"),
				HttpClient:         http.DefaultClient,
			}}
		BackendApi = backend.DiggerApi{
			DiggerHost: os.Getenv("DIGGER_HOSTNAME"),
			AuthToken:  os.Getenv("DIGGER_TOKEN"),
			HttpClient: http.DefaultClient,
		}
	} else {
		reportErrorAndExit("", "DIGGER_TOKEN not specified. You can get one at https://cloud.digger.dev, or self-manage a backend of Digger Community Edition (change DIGGER_HOSTNAME). You can also pass 'no-backend: true' option; in this case some of the features may not be available.", 1)
	}

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
	lock, err = locking.GetLock()
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
