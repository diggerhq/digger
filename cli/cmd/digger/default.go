package main

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/cli/pkg/github"
	spec2 "github.com/diggerhq/digger/cli/pkg/spec"
	"github.com/diggerhq/digger/cli/pkg/usage"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/policy"
	lib_spec "github.com/diggerhq/digger/libs/spec"
	"github.com/spf13/cobra"
	"log"
	"os"
	"runtime/debug"
)

var defaultCmd = &cobra.Command{
	Use: "default",
	Run: func(cmd *cobra.Command, args []string) {

		specStr := os.Getenv("DIGGER_RUN_SPEC")
		if specStr != "" {
			var spec lib_spec.Spec
			err := json.Unmarshal([]byte(specStr), &spec)
			if err != nil {
				usage.ReportErrorAndExit("", fmt.Sprintf("could not load spec json: %v", err), 1)
			}
			err = spec2.RunSpec(
				spec,
				lib_spec.VCSProviderBasic{},
				lib_spec.JobSpecProvider{},
				lib_spec.LockProvider{},
				lib_spec.ReporterProvider{},
				lib_spec.BackendApiProvider{},
				lib_spec.BasicPolicyProvider{},
				lib_spec.PlanStorageProvider{},
				comment_updater.CommentUpdaterProviderBasic{},
			)
			usage.ReportErrorAndExit(spec.VCS.Actor, "Successfully ran spec", 0)
		}

		var logLeader = "Unknown CI"
		ci := digger.DetectCI()

		switch ci {
		case digger.GitHub:
			logLeader = os.Getenv("GITHUB_ACTOR")
			github.GitHubCI(lock, policy.PolicyCheckerProviderBasic{}, BackendApi, ReportStrategy, dg_github.GithubServiceProviderBasic{}, comment_updater.CommentUpdaterProviderBasic{}, drift.DriftNotificationProviderBasic{})
		case digger.None:
			print("No CI detected.")
			os.Exit(10)
		}

		defer func() {
			if r := recover(); r != nil {
				log.Println(fmt.Sprintf("stacktrace from panic: \n" + string(debug.Stack())))
				err := usage.SendLogRecord(logLeader, fmt.Sprintf("Panic occurred. %s", r))
				if err != nil {
					log.Printf("Failed to send log record. %s\n", err)
				}
				os.Exit(1)
			}
		}()
	},
}

func init() {
	rootCmd.AddCommand(defaultCmd)
}
