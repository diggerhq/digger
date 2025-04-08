package gitlab

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/cli/pkg/usage"
	core_backend "github.com/diggerhq/digger/libs/backendapi"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/ci/gitlab"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	core_locking "github.com/diggerhq/digger/libs/locking"
	core_policy "github.com/diggerhq/digger/libs/policy"
	"github.com/diggerhq/digger/libs/scheduler"
	"log"
	"os"
)

func GitLabCI(lock core_locking.Lock, policyCheckerProvider core_policy.PolicyCheckerProvider, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy, githubServiceProvider dg_github.GithubServiceProvider, commentUpdaterProvider comment_updater.CommentUpdaterProvider, driftNotificationProvider drift.DriftNotificationProvider) {
	println("Using GitLab.")

	repoOwner := os.Getenv("CI_PROJECT_NAMESPACE")
	repoName := os.Getenv("CI_PROJECT_NAME")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	actor := "gitlab_user"
	if gitlabToken == "" {
		fmt.Println("GITLAB_TOKEN is empty")
	}

	repoFullName := fmt.Sprintf("%v/%v", repoOwner, repoName)
	diggerConfig, _, _, err := digger_config.LoadDiggerConfig("./", true, nil)
	if err != nil {
		usage.ReportErrorAndExit(repoOwner, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Printf("Digger digger_config read successfully\n")

	// default policy checker for backwards compatibility, will be overridden in orchestrator flow
	var policyChecker = core_policy.NoOpPolicyChecker{}

	currentDir, err := os.Getwd()
	if err != nil {
		usage.ReportErrorAndExit(repoName, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	gitLabContext, err := gitlab.ParseGitLabContext()
	if err != nil {
		fmt.Printf("failed to parse GitLab context. %s\n", err.Error())
		os.Exit(4)
	}

	// it's ok to not have merge request info if it has been merged
	//if (gitLabContext.MergeRequestIId == nil || len(gitLabContext.OpenMergeRequests) == 0) && gitLabContext.EventType != "merge_request_merge" {
	//	fmt.Println("No merge request found.")
	//	os.Exit(0)
	//}

	gitlabService, err := gitlab.NewGitLabService(gitlabToken, gitLabContext, "")
	if err != nil {
		fmt.Printf("failed to initialise GitLab service, %v", err)
		os.Exit(4)
	}

	runningMode := os.Getenv("INPUT_DIGGER_MODE")

	if runningMode == "drift-detection" {

		for _, projectConfig := range diggerConfig.Projects {
			if !projectConfig.DriftDetection {
				continue
			}
			workflow := diggerConfig.Workflows[projectConfig.Workflow]

			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)

			StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(projectConfig)

			stateArn, cmdArn := "", ""
			if projectConfig.AwsRoleToAssume != nil {
				if projectConfig.AwsRoleToAssume.State != "" {
					stateArn = projectConfig.AwsRoleToAssume.State
				}

				if projectConfig.AwsRoleToAssume.Command != "" {
					cmdArn = projectConfig.AwsRoleToAssume.Command
				}
			}

			job := scheduler.Job{
				ProjectName:        projectConfig.Name,
				ProjectDir:         projectConfig.Dir,
				ProjectWorkspace:   projectConfig.Workspace,
				Terragrunt:         projectConfig.Terragrunt,
				OpenTofu:           projectConfig.OpenTofu,
				Pulumi:             projectConfig.Pulumi,
				Commands:           []string{"digger drift-detect"},
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				RequestedBy:        actor,
				Namespace:          repoFullName,
				EventName:          "drift-detect",
				StateEnvProvider:   StateEnvProvider,
				CommandEnvProvider: CommandEnvProvider,
				StateRoleArn:       stateArn,
				CommandRoleArn:     cmdArn,
			}

			notification, err := driftNotificationProvider.Get(gitlabService)
			if err != nil {
				usage.ReportErrorAndExit(repoFullName, fmt.Sprintf("could not get drift notification type: %v", err), 8)
			}

			err = digger.RunJob(job, repoFullName, actor, gitlabService, policyChecker, nil, backendApi, &notification, currentDir)
			if err != nil {
				usage.ReportErrorAndExit(repoOwner, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		}
	} else {
		usage.ReportErrorAndExit(repoOwner, fmt.Sprintf("unrecognised input mode: %v", runningMode), 1)
	}
	println("Commands executed successfully")

	usage.ReportErrorAndExit(repoOwner, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			usage.ReportErrorAndExit(repoOwner, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()
}
