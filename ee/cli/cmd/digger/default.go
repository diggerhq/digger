package main

import (
	"crypto/fips140"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/github"
	spec2 "github.com/diggerhq/digger/cli/pkg/spec"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/ee/cli/pkg/comment_updater"
	"github.com/diggerhq/digger/ee/cli/pkg/drift"
	github2 "github.com/diggerhq/digger/ee/cli/pkg/github"
	"github.com/diggerhq/digger/ee/cli/pkg/gitlab"
	"github.com/diggerhq/digger/ee/cli/pkg/policy"
	"github.com/diggerhq/digger/ee/cli/pkg/vcs"
	comment_summary "github.com/diggerhq/digger/libs/comment_utils/summary"
	lib_spec "github.com/diggerhq/digger/libs/spec"
	"github.com/spf13/cobra"
)

var defaultCmd = &cobra.Command{
	Use: "default",
	Run: func(cmd *cobra.Command, args []string) {
		specStr := os.Getenv("DIGGER_RUN_SPEC")
		log.Printf("Fips140 enabled in build: %v", fips140.Enabled())
		if specStr != "" {
			var spec lib_spec.Spec
			err := json.Unmarshal([]byte(specStr), &spec)
			if err != nil {
				usage.ReportErrorAndExit("", fmt.Sprintf("could not load spec json: %v", err), 1)
			}
			if spec.SpecType == lib_spec.SpecTypeManualJob {
				err = spec2.RunSpecManualCommand(
					spec,
					vcs.VCSProviderAdvanced{},
					lib_spec.JobSpecProvider{},
					lib_spec.LockProvider{},
					lib_spec.ReporterProvider{},
					lib_spec.BackendApiProvider{},
					policy.AdvancedPolicyProvider{},
					lib_spec.PlanStorageProvider{},
					comment_summary.CommentUpdaterProviderBasic{},
				)
			} else {
				err = spec2.RunSpec(
					spec,
					vcs.VCSProviderAdvanced{},
					lib_spec.JobSpecProvider{},
					lib_spec.LockProvider{},
					lib_spec.ReporterProvider{},
					lib_spec.BackendApiProvider{},
					policy.AdvancedPolicyProvider{},
					lib_spec.PlanStorageProvider{},
					lib_spec.VariablesProvider{},
					comment_summary.CommentUpdaterProviderBasic{},
				)
			}
			usage.ReportErrorAndExit(spec.VCS.Actor, "Successfully ran spec", 0)
		}

		logLeader := "Unknown CI"
		ci := digger.DetectCI()

		switch ci {
		case digger.GitHub:
			logLeader = os.Getenv("GITHUB_ACTOR")
			github.GitHubCI(lock, policy.PolicyCheckerProviderAdvanced{}, BackendApi, ReportStrategy, github2.GithubServiceProviderAdvanced{}, comment_updater.CommentUpdaterProviderAdvanced{}, drift.DriftNotificationProviderAdvanced{})
		case digger.GitLab:
			log.Printf("gitlab CI detected")
			gitlab.GitLabCI(lock, policy.PolicyCheckerProviderAdvanced{}, BackendApi, ReportStrategy, github2.GithubServiceProviderAdvanced{}, comment_updater.CommentUpdaterProviderAdvanced{}, drift.DriftNotificationProviderAdvanced{})
		case digger.None:
			print("No CI detected.")
			os.Exit(10)
		}

		defer func() {
			if r := recover(); r != nil {
				log.Println("stacktrace from panic: \n" + string(debug.Stack()))
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
