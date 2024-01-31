package main

import (
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	core_reporting "github.com/diggerhq/digger/cli/pkg/core/reporting"
	github_pkg "github.com/diggerhq/digger/cli/pkg/github"
	"github.com/diggerhq/digger/cli/pkg/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

var vip *viper.Viper

func plan(actor string, projectName string, repoNamespace string, lock core_locking.Lock, policyChecker core_policy.Checker, reporter core_reporting.Reporter, prService orchestrator.PullRequestService, orgService orchestrator.OrgService, backendApi core_backend.Api) {
	exec(actor, projectName, repoNamespace, "digger plan", lock, policyChecker, prService, orgService, reporter, backendApi)
}

var planCmd = &cobra.Command{
	Use:   "plan project_name [flags]",
	Short: "Plan a project, if no project specified it will plan for all projects",
	Long:  `Plan a project, if no project specified it will plan for all projects`,
	Run: func(cmd *cobra.Command, args []string) {
		var runConfig RunConfig
		vip.Unmarshal(&runConfig)
		var prService orchestrator.PullRequestService
		var orgService orchestrator.OrgService
		var reporter core_reporting.Reporter

		switch runConfig.Reporter {
		case "github":
			splitRepositoryName := strings.Split(runConfig.RepoNamespace, "/")
			repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
			prService = orchestrator_github.NewGitHubService(runConfig.GithubToken, repositoryName, repoOwner)
			orgService = orchestrator_github.NewGitHubService(runConfig.GithubToken, runConfig.RepoNamespace, runConfig.Actor)
			reporter = &reporting.CiReporter{
				CiService:         prService,
				ReportStrategy:    ReportStrategy,
				PrNumber:          runConfig.PRNumber,
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

		}

		plan(runConfig.Actor, args[0], runConfig.RepoNamespace, lock, PolicyChecker, reporter, prService, orgService, BackendApi)
	},
}

func init() {
	flags := []pflag.Flag{
		{Name: "github-token", Usage: "The namespace of this repo"},
		{Name: "repo-namespace", Usage: "The namespace of this repo"},
		{Name: "actor", Usage: "The actor of this command"},
		{Name: "reporter", Usage: "The reporter to use (defaults to stdout)"},
		{Name: "pr-number", Usage: "The PR number for reporting"},
		{Name: "comment-id", Usage: "The PR comment for reporting"},
	}

	vip = viper.New()
	vip.SetEnvPrefix("DIGGER")
	vip.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	vip.AutomaticEnv()

	for _, flag := range flags {
		planCmd.Flags().String(flag.Name, "", flag.Usage)
		vip.BindPFlag(flag.Name, planCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(planCmd)
}
