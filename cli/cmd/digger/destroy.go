package main

import (
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	core_reporting "github.com/diggerhq/digger/cli/pkg/core/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

var vipDestroy *viper.Viper

func destroy(actor string, projectName string, repoNamespace string, prNumber int, lock core_locking.Lock, policyChecker core_policy.Checker, reporter core_reporting.Reporter, prService orchestrator.PullRequestService, orgService orchestrator.OrgService, backendApi core_backend.Api) {
	exec(actor, projectName, repoNamespace, "digger destroy", prNumber, lock, policyChecker, prService, orgService, reporter, backendApi)
}

var destroyCmd = &cobra.Command{
	Use:   "destroy project_name [flags]",
	Short: "Destroy a project",
	Long:  `Destroy a project`,
	Run: func(cmd *cobra.Command, args []string) {
		var runConfig RunConfig
		vipDestroy.Unmarshal(&runConfig)

		prService, orgService, reporter, err := runConfig.GetServices()
		if err != nil {
			reportErrorAndExit(runConfig.Actor, "Unrecognised reporter: "+runConfig.Reporter, 1)
		}

		destroy(runConfig.Actor, args[0], runConfig.RepoNamespace, runConfig.PRNumber, lock, PolicyChecker, *reporter, *prService, *orgService, BackendApi)
	},
}

func init() {
	flags := []pflag.Flag{
		{Name: "github-token", Usage: "Github token (for github reporter)"},
		{Name: "repo-namespace", Usage: "The namespace of this repo"},
		{Name: "actor", Usage: "The actor of this command"},
		{Name: "reporter", Usage: "The reporter to use (defaults to stdout)"},
		{Name: "pr-number", Usage: "The PR number for reporting"},
		{Name: "comment-id", Usage: "The PR comment for reporting"},
	}

	vipDestroy = viper.New()
	vipDestroy.SetEnvPrefix("DIGGER")
	vipDestroy.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	vipDestroy.AutomaticEnv()

	for _, flag := range flags {
		destroyCmd.Flags().String(flag.Name, "", flag.Usage)
		vipDestroy.BindPFlag(flag.Name, destroyCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(destroyCmd)
}
