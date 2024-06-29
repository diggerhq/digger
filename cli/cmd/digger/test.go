package main

import (
	"strings"

	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var vipTest *viper.Viper

func test(actor string, projectName string, repoNamespace string, prNumber int, lock core_locking.Lock, policyChecker core_policy.Checker, reporter reporting.Reporter, prService orchestrator.PullRequestService, orgService orchestrator.OrgService, backendApi core_backend.Api) {
	exec(actor, projectName, repoNamespace, "digger test", prNumber, lock, policyChecker, prService, orgService, reporter, backendApi)
}

var testCmd = &cobra.Command{
	Use:   "test project_name [flags]",
	Short: "test a project, if no project specified it will plan for all projects",
	Long:  `test a project, if no project specified it will plan for all projects`,
	Run: func(cmd *cobra.Command, args []string) {
		var runConfig RunConfig
		vipTest.Unmarshal(&runConfig)

		prService, orgService, reporter, err := runConfig.GetServices()
		if err != nil {
			reportErrorAndExit(runConfig.Actor, "Unrecognised reporter: "+runConfig.Reporter, 1)
		}

		test(runConfig.Actor, args[0], runConfig.RepoNamespace, runConfig.PRNumber, lock, PolicyChecker, *reporter, *prService, *orgService, BackendApi)
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

	vipTest = viper.New()
	vipTest.SetEnvPrefix("DIGGER")
	vipTest.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	vipTest.AutomaticEnv()

	for _, flag := range flags {
		applyCmd.Flags().String(flag.Name, "", flag.Usage)
		vipTest.BindPFlag(flag.Name, applyCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(testCmd)
}
