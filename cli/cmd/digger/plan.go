package main

import (
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

var vipPlan *viper.Viper

func plan(actor string, projectName string, repoNamespace string, prNumber int, lock core_locking.Lock, policyChecker core_policy.Checker, reporter reporting.Reporter, prService orchestrator.PullRequestService, orgService orchestrator.OrgService, backendApi core_backend.Api) {
	exec(actor, projectName, repoNamespace, "digger plan", prNumber, lock, policyChecker, prService, orgService, reporter, backendApi)
}

var planCmd = &cobra.Command{
	Use:   "plan project_name [flags]",
	Short: "Plan a project, if no project specified it will plan for all projects",
	Long:  `Plan a project, if no project specified it will plan for all projects`,
	Run: func(cmd *cobra.Command, args []string) {
		var runConfig RunConfig
		vipPlan.Unmarshal(&runConfig)

		prService, orgService, reporter, err := runConfig.GetServices()
		if err != nil {
			reportErrorAndExit(runConfig.Actor, "Unrecognised reporter: "+runConfig.Reporter, 1)
		}

		plan(runConfig.Actor, args[0], runConfig.RepoNamespace, runConfig.PRNumber, lock, PolicyChecker, *reporter, *prService, *orgService, BackendApi)
	},
}

func init() {
	flags := []pflag.Flag{
		{Name: "github-token", Usage: "Github token (for github reporter)"},
		{Name: "bitbucket-token", Usage: "Bitbucket token (for bitbucket reporter)"},
		{Name: "repo-namespace", Usage: "The namespace of this repo"},
		{Name: "actor", Usage: "The actor of this command"},
		{Name: "reporter", Usage: "The reporter to use (defaults to stdout)"},
		{Name: "pr-number", Usage: "The PR number for reporting"},
		{Name: "comment-id", Usage: "The PR comment for reporting"},
	}

	vipPlan = viper.New()
	vipPlan.SetEnvPrefix("DIGGER")
	vipPlan.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	vipPlan.AutomaticEnv()

	for _, flag := range flags {
		planCmd.Flags().String(flag.Name, "", flag.Usage)
		vipPlan.BindPFlag(flag.Name, planCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(planCmd)
}
