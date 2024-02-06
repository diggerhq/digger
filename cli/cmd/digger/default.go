package main

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/azure"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/spf13/cobra"
	"log"
	"os"
	"runtime/debug"
)

var defaultCmd = &cobra.Command{
	Use: "default",
	Run: func(cmd *cobra.Command, args []string) {
		var logLeader = "Unknown CI"
		ci := digger.DetectCI()

		switch ci {
		case digger.GitHub:
			logLeader = os.Getenv("GITHUB_ACTOR")
			gitHubCI(lock, PolicyChecker, BackendApi, ReportStrategy)
		case digger.GitLab:
			logLeader = os.Getenv("CI_PROJECT_NAME")
			gitLabCI(lock, PolicyChecker, BackendApi, ReportStrategy)
		case digger.Azure:
			// This should be refactored in the future because in this way the parsing
			// is done twice, both here and inside azureCI, a better solution might be
			// to encapsulate it into a method on the azure package and then grab the
			// value here and pass it into the azureCI call.
			azureContext := os.Getenv("AZURE_CONTEXT")
			parsedAzureContext, _ := azure.GetAzureReposContext(azureContext)
			logLeader = parsedAzureContext.BaseUrl
			azureCI(lock, PolicyChecker, BackendApi, ReportStrategy)
		case digger.BitBucket:
			logLeader = os.Getenv("BITBUCKET_STEP_TRIGGERER_UUID")
			bitbucketCI(lock, PolicyChecker, BackendApi, ReportStrategy)
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
