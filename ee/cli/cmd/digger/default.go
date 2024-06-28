package main

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/github"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/ee/cli/pkg/comment_updater"
	"github.com/diggerhq/digger/ee/cli/pkg/drift"
	github2 "github.com/diggerhq/digger/ee/cli/pkg/github"
	"github.com/diggerhq/digger/ee/cli/pkg/policy"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/spf13/cobra"
	"log"
	"os"
	"runtime/debug"
	"strconv"
)

var defaultCmd = &cobra.Command{
	Use: "default",
	Run: func(cmd *cobra.Command, args []string) {
		var logLeader = "Unknown CI"
		ci := digger.DetectCI()

		switch ci {
		case digger.GitHub:
			logLeader = os.Getenv("GITHUB_ACTOR")

			spec := spec.Spec{
				JobId:     "",
				CommentId: strconv.FormatInt(commentId, 10),
				Job:       jobSpec,
				Reporter: spec.ReporterSpec{
					ReportingStrategy: "comments_per_run",
					ReporterType:      "lazy",
				},
				Lock: spec.LockSpec{
					LockType: "noop",
				},
				Backend: spec.BackendSpec{
					BackendType: "noop",
				},
				VCS: spec.VcsSpec{
					VcsType:   "github",
					Actor:     jobSpec.RequestedBy,
					RepoOwner: repoOwner,
					RepoName:  repoName,
				},
				Policy: spec.PolicySpec{
					PolicyType: "noop",
				},
			}

			github.GitHubCI(lock, policy.PolicyCheckerProviderAdvanced{}, BackendApi, ReportStrategy, github2.GithubServiceProviderAdvanced{}, comment_updater.CommentUpdaterProviderAdvanced{}, drift.DriftNotificationProviderAdvanced{})
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
