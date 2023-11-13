package digger

import (
	"errors"
	"fmt"
	config "github.com/diggerhq/digger/libs/digger_config"
	orchestrator "github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/pkg/core/backend"
	"github.com/diggerhq/digger/pkg/core/execution"
	core_locking "github.com/diggerhq/digger/pkg/core/locking"
	"github.com/diggerhq/digger/pkg/core/policy"
	core_reporting "github.com/diggerhq/digger/pkg/core/reporting"
	"github.com/diggerhq/digger/pkg/core/runners"
	"github.com/diggerhq/digger/pkg/core/storage"
	"github.com/diggerhq/digger/pkg/core/terraform"
	"github.com/diggerhq/digger/pkg/core/utils"
	"github.com/diggerhq/digger/pkg/locking"
	"github.com/diggerhq/digger/pkg/notification"
	"github.com/diggerhq/digger/pkg/reporting"
	"github.com/diggerhq/digger/pkg/usage"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dominikbraun/graph"
)

type CIName string

const (
	None      = CIName("")
	GitHub    = CIName("github")
	GitLab    = CIName("gitlab")
	BitBucket = CIName("bitbucket")
	Azure     = CIName("azure")
)

func (ci CIName) String() string {
	return string(ci)
}

func DetectCI() CIName {

	notEmpty := func(key string) bool {
		return os.Getenv(key) != ""
	}

	if notEmpty("GITHUB_ACTIONS") {
		return GitHub
	}
	if notEmpty("GITLAB_CI") {
		return GitLab
	}
	if notEmpty("BITBUCKET_BUILD_NUMBER") {
		return BitBucket
	}
	if notEmpty("AZURE_CI") {
		return Azure
	}
	return None

}

func RunJobs(
	jobs []orchestrator.Job,
	prService orchestrator.PullRequestService,
	orgService orchestrator.OrgService,
	lock core_locking.Lock,
	reporter core_reporting.Reporter,
	planStorage storage.PlanStorage,
	policyChecker policy.Checker,
	backendApi backend.Api,
	workingDir string,
) (bool, bool, error) {
	runStartedAt := time.Now()

	appliesPerProject := make(map[string]bool)

	for _, job := range jobs {
		splits := strings.Split(job.Namespace, "/")
		SCMOrganisation := splits[0]
		SCMrepository := splits[1]

		for _, command := range job.Commands {
			allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, command, job.PullRequestNumber, job.RequestedBy)

			if err != nil {
				return false, false, fmt.Errorf("error checking policy: %v", err)
			}

			if !allowedToPerformCommand {
				msg := reportPolicyError(job.ProjectName, job.RequestedBy, command, reporter)
				log.Printf("Skipping command ... %v for project %v", command, job.ProjectName)
				log.Println(msg)
				appliesPerProject[job.ProjectName] = false
				continue
			}

			output, err := run(command, job, policyChecker, orgService, SCMOrganisation, SCMrepository, job.RequestedBy, reporter, lock, prService, job.Namespace, workingDir, planStorage, appliesPerProject)

			if err != nil {
				reportErr := backendApi.ReportProjectRun(SCMOrganisation+"-"+SCMrepository, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, output)
				if reportErr != nil {
					log.Printf("error reporting project run err: %v.\n", reportErr)
				}
				return false, false, fmt.Errorf("error while running command: %v", err)
			}
			err = backendApi.ReportProjectRun(SCMOrganisation+"-"+SCMrepository, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, output)
			if err != nil {
				log.Printf("Error reporting project run: %v", err)
			}
		}
	}

	allAppliesSuccess := true
	for _, success := range appliesPerProject {
		if !success {
			allAppliesSuccess = false
		}
	}

	atLeastOneApply := len(appliesPerProject) > 0

	return allAppliesSuccess, atLeastOneApply, nil
}

func reportPolicyError(projectName string, command string, requestedBy string, reporter core_reporting.Reporter) string {
	msg := fmt.Sprintf("User %s is not allowed to perform action: %s. Check your policies :x:", requestedBy, command)
	if reporter.SupportsMarkdown() {
		err := reporter.Report(msg, utils.AsCollapsibleComment(fmt.Sprintf("Policy violation for <b>%v - %v</b>", projectName, command)))
		if err != nil {
			log.Printf("Error publishing comment: %v", err)
		}
	} else {
		err := reporter.Report(msg, utils.AsComment(fmt.Sprintf("Policy violation for %v - %v", projectName, command)))
		if err != nil {
			log.Printf("Error publishing comment: %v", err)
		}
	}
	return msg
}

func run(command string, job orchestrator.Job, policyChecker policy.Checker, orgService orchestrator.OrgService, SCMOrganisation string, SCMrepository string, requestedBy string, reporter core_reporting.Reporter, lock core_locking.Lock, prService orchestrator.PullRequestService, projectNamespace string, workingDir string, planStorage storage.PlanStorage, appliesPerProject map[string]bool) (string, error) {
	log.Printf("Running '%s' for project '%s' (workflow: %s)\n", command, job.ProjectName, job.ProjectWorkflow)

	allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, command, job.PullRequestNumber, requestedBy)

	if err != nil {
		return "error checking policy", fmt.Errorf("error checking policy: %v", err)
	}

	if !allowedToPerformCommand {
		msg := reportPolicyError(job.ProjectName, command, requestedBy, reporter)
		log.Println(msg)
		return msg, errors.New(msg)
	}

	projectLock := &locking.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        prService,
		ProjectName:      job.ProjectName,
		ProjectNamespace: projectNamespace,
		PrNumber:         *job.PullRequestNumber,
	}

	var terraformExecutor terraform.TerraformExecutor
	projectPath := path.Join(workingDir, job.ProjectDir)
	if job.Terragrunt {
		terraformExecutor = terraform.Terragrunt{WorkingDir: projectPath}
	} else if job.OpenTofu {
		terraformExecutor = terraform.OpenTofu{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
	} else {
		terraformExecutor = terraform.Terraform{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
	}

	commandRunner := runners.CommandRunner{}
	planPathProvider := execution.ProjectPathProvider{
		ProjectPath:      projectPath,
		ProjectNamespace: projectNamespace,
		ProjectName:      job.ProjectName,
	}
	diggerExecutor := execution.LockingExecutorWrapper{
		ProjectLock: projectLock,
		Executor: execution.DiggerExecutor{
			ProjectNamespace:  projectNamespace,
			ProjectName:       job.ProjectName,
			ProjectPath:       projectPath,
			StateEnvVars:      job.StateEnvVars,
			CommandEnvVars:    job.CommandEnvVars,
			ApplyStage:        job.ApplyStage,
			PlanStage:         job.PlanStage,
			CommandRunner:     commandRunner,
			TerraformExecutor: terraformExecutor,
			Reporter:          reporter,
			PlanStorage:       planStorage,
			PlanPathProvider:  planPathProvider,
		},
	}

	switch command {
	case "digger plan":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "plan")
		if err != nil {
			log.Printf("failed to send usage report. %v", err)
		}
		err = prService.SetStatus(*job.PullRequestNumber, "pending", job.ProjectName+"/plan")
		if err != nil {
			msg := fmt.Sprintf("Failed to set PR status. %v", err)
			return msg, fmt.Errorf(msg)
		}
		planPerformed, isNonEmptyPlan, plan, planJsonOutput, err := diggerExecutor.Plan()

		if err != nil {
			msg := fmt.Sprintf("Failed to run digger plan command. %v", err)
			log.Printf(msg)
			err := prService.SetStatus(*job.PullRequestNumber, "failure", job.ProjectName+"/plan")
			if err != nil {
				msg := fmt.Sprintf("Failed to set PR status. %v", err)
				return msg, fmt.Errorf(msg)
			}
			return msg, fmt.Errorf(msg)
		} else if planPerformed {
			reportTerraformPlanOutput(reporter, projectLock.LockId(), plan)
			if isNonEmptyPlan {
				planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, job.ProjectName, planJsonOutput)
				if err != nil {
					msg := fmt.Sprintf("Failed to validate plan. %v", err)
					log.Printf(msg)
					return msg, fmt.Errorf(msg)
				}
				var planPolicyFormatter func(report string) string
				summary := fmt.Sprintf("Terraform plan validation check (%v)", job.ProjectName)
				if reporter.SupportsMarkdown() {
					planPolicyFormatter = utils.AsCollapsibleComment(summary)
				} else {
					planPolicyFormatter = utils.AsComment(summary)
				}
				if !planIsAllowed {
					planReportMessage := "Terraform plan failed validation checks :x:<br>"
					preformattedMessaged := make([]string, 0)
					for _, message := range messages {
						preformattedMessaged = append(preformattedMessaged, fmt.Sprintf("    %v", message))
					}
					planReportMessage = planReportMessage + strings.Join(preformattedMessaged, "<br>")
					err = reporter.Report(planReportMessage, planPolicyFormatter)

					if err != nil {
						log.Printf("Failed to report plan. %v", err)
					}
					msg := fmt.Sprintf("Plan is not allowed")
					log.Printf(msg)
					return msg, fmt.Errorf(msg)
				} else {
					err := reporter.Report("Terraform plan validation checks succeeded :white_check_mark:", planPolicyFormatter)
					if err != nil {
						log.Printf("Failed to report plan. %v", err)
					}
				}
			}
			err := prService.SetStatus(*job.PullRequestNumber, "success", job.ProjectName+"/plan")
			if err != nil {
				msg := fmt.Sprintf("Failed to set PR status. %v", err)
				return msg, fmt.Errorf(msg)
			}
			return plan, nil
		}
	case "digger apply":
		appliesPerProject[job.ProjectName] = false
		err := usage.SendUsageRecord(requestedBy, job.EventName, "apply")
		if err != nil {
			log.Printf("failed to send usage report. %v", err)
		}
		err = prService.SetStatus(*job.PullRequestNumber, "pending", job.ProjectName+"/apply")
		if err != nil {
			msg := fmt.Sprintf("Failed to set PR status. %v", err)
			return msg, fmt.Errorf(msg)
		}

		isMerged, err := prService.IsMerged(*job.PullRequestNumber)
		if err != nil {
			msg := fmt.Sprintf("Failed to check if PR is merged. %v", err)
			return msg, fmt.Errorf(msg)
		}

		// this might go into some sort of "appliability" plugin later
		isMergeable, err := prService.IsMergeable(*job.PullRequestNumber)
		if err != nil {
			msg := fmt.Sprintf("Failed to check if PR is mergeable. %v", err)
			return msg, fmt.Errorf(msg)
		}
		log.Printf("PR status, mergeable: %v, merged: %v\n", isMergeable, isMerged)
		if !isMergeable && !isMerged {
			comment := reportApplyMergeabilityError(reporter)

			return comment, fmt.Errorf(comment)
		} else {
			applyPerformed, output, err := diggerExecutor.Apply()
			if err != nil {
				log.Printf("Failed to run digger apply command. %v", err)
				err := prService.SetStatus(*job.PullRequestNumber, "failure", job.ProjectName+"/apply")
				if err != nil {
					msg := fmt.Sprintf("Failed to set PR status. %v", err)
					return msg, fmt.Errorf(msg)
				}
				msg := fmt.Sprintf("Failed to run digger apply command. %v", err)
				return msg, fmt.Errorf(msg)
			} else if applyPerformed {
				err := prService.SetStatus(*job.PullRequestNumber, "success", job.ProjectName+"/apply")
				if err != nil {
					msg := fmt.Sprintf("Failed to set PR status. %v", err)
					return msg, fmt.Errorf(msg)
				}
				appliesPerProject[job.ProjectName] = true
			}
			return output, nil
		}
	case "digger unlock":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "unlock")
		if err != nil {
			log.Printf("failed to send usage report. %v", err)
		}
		err = diggerExecutor.Unlock()
		if err != nil {
			msg := fmt.Sprintf("Failed to unlock project. %v", err)
			return msg, fmt.Errorf(msg)
		}

		if planStorage != nil {
			err = planStorage.DeleteStoredPlan(planPathProvider.StoredPlanFilePath())
			if err != nil {
				log.Printf("failed to delete stored plan file '%v':  %v", planPathProvider.StoredPlanFilePath(), err)
			}
		}
	case "digger lock":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "lock")
		if err != nil {
			log.Printf("failed to send usage report. %v", err)
		}
		err = diggerExecutor.Lock()
		if err != nil {
			msg := fmt.Sprintf("Failed to lock project. %v", err)
			return msg, fmt.Errorf(msg)
		}

	case "digger drift-detect":
		plan, err := runDriftDetection(policyChecker, SCMOrganisation, SCMrepository, job.ProjectName, requestedBy, job.EventName, diggerExecutor)
		if err != nil {
			msg := fmt.Sprintf("Failed to run drift detection. %v", err)
			return msg, fmt.Errorf(msg)
		}
		return plan, nil
	default:
		msg := fmt.Sprintf("Command '%s' is not supported", command)
		return msg, fmt.Errorf(msg)
	}
	return "", nil
}

func reportApplyMergeabilityError(reporter core_reporting.Reporter) string {
	comment := "cannot perform Apply since the PR is not currently mergeable"
	log.Println(comment)

	if reporter.SupportsMarkdown() {
		err := reporter.Report(comment, utils.AsCollapsibleComment("Apply error"))
		if err != nil {
			log.Printf("error publishing comment: %v\n", err)
		}
	} else {
		err := reporter.Report(comment, utils.AsComment("Apply error"))
		if err != nil {
			log.Printf("error publishing comment: %v\n", err)
		}
	}
	return comment
}

func reportTerraformPlanOutput(reporter core_reporting.Reporter, projectId string, plan string) {
	var formatter func(string) string

	if reporter.SupportsMarkdown() {
		formatter = utils.GetTerraformOutputAsCollapsibleComment("Plan for <b>" + projectId + "</b>")
	} else {
		formatter = utils.GetTerraformOutputAsComment("Plan for " + projectId)
	}

	err := reporter.Report(plan, formatter)
	if err != nil {
		log.Printf("Failed to report plan. %v", err)
	}
}

func RunJob(
	job orchestrator.Job,
	repo string,
	requestedBy string,
	orgService orchestrator.OrgService,
	policyChecker policy.Checker,
	planStorage storage.PlanStorage,
	backendApi backend.Api,
	workingDir string,
) error {
	runStartedAt := time.Now()
	splits := strings.Split(repo, "/")
	SCMOrganisation := splits[0]
	SCMrepository := splits[1]
	log.Printf("Running '%s' for project '%s'\n", job.Commands, job.ProjectName)

	for _, command := range job.Commands {

		allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, nil, SCMOrganisation, SCMrepository, job.ProjectName, command, nil, requestedBy)

		if err != nil {
			return fmt.Errorf("error checking policy: %v", err)
		}

		if !allowedToPerformCommand {
			msg := fmt.Sprintf("User %s is not allowed to perform action: %s. Check your policies", requestedBy, command)
			if err != nil {
				log.Printf("Error publishing comment: %v", err)
			}
			log.Println(msg)
			err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FORBIDDEN", command, msg)
			if err != nil {
				log.Printf("Error reporting run: %v", err)
			}
			return errors.New(msg)
		}
		var terraformExecutor terraform.TerraformExecutor
		projectPath := path.Join(workingDir, job.ProjectDir)
		if job.Terragrunt {
			terraformExecutor = terraform.Terragrunt{WorkingDir: projectPath}
		} else if job.OpenTofu {
			terraformExecutor = terraform.OpenTofu{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
		} else {
			terraformExecutor = terraform.Terraform{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
		}

		commandRunner := runners.CommandRunner{}

		planPathProvider := execution.ProjectPathProvider{
			ProjectPath:      projectPath,
			ProjectNamespace: repo,
			ProjectName:      job.ProjectName,
		}

		diggerExecutor := execution.DiggerExecutor{
			ProjectNamespace:  repo,
			ProjectName:       job.ProjectName,
			ProjectPath:       projectPath,
			StateEnvVars:      job.StateEnvVars,
			CommandEnvVars:    job.CommandEnvVars,
			ApplyStage:        job.ApplyStage,
			PlanStage:         job.PlanStage,
			CommandRunner:     commandRunner,
			Reporter:          &reporting.StdOutReporter{},
			TerraformExecutor: terraformExecutor,
			PlanStorage:       planStorage,
			PlanPathProvider:  planPathProvider,
		}

		switch command {
		case "digger plan":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "plan")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, _, plan, planJsonOutput, err := diggerExecutor.Plan()
			if err != nil {
				msg := fmt.Sprintf("Failed to run digger plan command. %v", err)
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting run: %v", err)
				}
				return fmt.Errorf(msg)
			}
			planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, job.ProjectName, planJsonOutput)
			log.Printf(strings.Join(messages, "\n"))
			if err != nil {
				msg := fmt.Sprintf("Failed to validate plan %v", err)
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting run: %v", err)
				}
				return fmt.Errorf(msg)
			}
			if !planIsAllowed {
				msg := fmt.Sprintf("Plan is not allowed")
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting run: %v", err)
				}
				return fmt.Errorf(msg)
			} else {
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, plan)
				if err != nil {
					log.Printf("Error reporting run: %v", err)
				}
			}

		case "digger apply":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "apply")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, output, err := diggerExecutor.Apply()
			if err != nil {
				msg := fmt.Sprintf("Failed to run digger apply command. %v", err)
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting run: %v", err)
				}
				return fmt.Errorf(msg)
			}
			err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, output)
			if err != nil {
				log.Printf("Error reporting run: %v", err)
			}
		case "digger destroy":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "destroy")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, err = diggerExecutor.Destroy()
			if err != nil {
				log.Printf("Failed to run digger destroy command. %v", err)
				return fmt.Errorf("failed to run digger apply command. %v", err)
			}

		case "digger drift-detect":
			output, err := runDriftDetection(policyChecker, SCMOrganisation, SCMrepository, job.ProjectName, requestedBy, job.EventName, diggerExecutor)
			if err != nil {
				return fmt.Errorf("failed to run digger drift-detect command. %v", err)
			}
			err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, output)
			if err != nil {
				log.Printf("Error reporting run: %v", err)
			}
		}

	}
	return nil
}

func runDriftDetection(policyChecker policy.Checker, SCMOrganisation string, SCMrepository string, projectName string, requestedBy string, eventName string, diggerExecutor execution.Executor) (string, error) {
	err := usage.SendUsageRecord(requestedBy, eventName, "drift-detect")
	if err != nil {
		log.Printf("Failed to send usage report. %v", err)
	}
	policyEnabled, err := policyChecker.CheckDriftPolicy(SCMOrganisation, SCMrepository, projectName)
	if err != nil {
		msg := fmt.Sprintf("failed to check drift policy. %v", err)
		log.Printf(msg)
		return msg, fmt.Errorf(msg)
	}

	if !policyEnabled {
		msg := "skipping this drift application since it is not enabled for this project"
		log.Printf(msg)
		return msg, nil
	}
	planPerformed, nonEmptyPlan, plan, _, err := diggerExecutor.Plan()
	if err != nil {
		msg := fmt.Sprintf("failed to run digger plan command. %v", err)
		log.Printf(msg)
		return msg, fmt.Errorf(msg)
	}

	if planPerformed && nonEmptyPlan {
		slackNotificationUrl := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL")

		if slackNotificationUrl == "" {
			msg := "no INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL set, not sending notification"
			log.Printf(msg)
			return msg, fmt.Errorf(msg)
		}
		// send notification
		notificationMessage := fmt.Sprintf(":bangbang: Drift detected in digger project %v details below: \n\n```\n%v\n```", projectName, plan)
		notification := notification.SlackNotification{Url: slackNotificationUrl}
		err := notification.Send(notificationMessage)
		if err != nil {
			log.Printf("Erorr sending drift notification: %v", err)
		}
	} else if planPerformed && !nonEmptyPlan {
		log.Printf("No drift detected")
	} else {
		log.Printf("No plan performed")
	}
	return plan, nil
}

func SortedCommandsByDependency(project []orchestrator.Job, dependencyGraph *graph.Graph[string, config.Project]) []orchestrator.Job {
	var sortedCommands []orchestrator.Job
	sortedGraph, err := graph.StableTopologicalSort(*dependencyGraph, func(s string, s2 string) bool {
		return s < s2
	})
	if err != nil {
		log.Printf("dependencyGraph: %v", dependencyGraph)
		log.Fatalf("failed to sort commands by dependency, %v", err)
	}
	for _, node := range sortedGraph {
		for _, command := range project {
			if command.ProjectName == node {
				sortedCommands = append(sortedCommands, command)
			}
		}
	}
	return sortedCommands
}

func MergePullRequest(ciService orchestrator.PullRequestService, prNumber int) {
	time.Sleep(5 * time.Second)

	// CheckAccessPolicy if it was manually merged
	isMerged, err := ciService.IsMerged(prNumber)
	if err != nil {
		log.Fatalf("error checking if PR is merged: %v", err)
	}

	if !isMerged {
		combinedStatus, err := ciService.GetCombinedPullRequestStatus(prNumber)

		if err != nil {
			log.Fatalf("failed to get combined status, %v", err)
		}

		if combinedStatus != "success" {
			log.Fatalf("PR is not mergeable. Status: %v", combinedStatus)
		}

		prIsMergeable, err := ciService.IsMergeable(prNumber)

		if err != nil {
			log.Fatalf("failed to check if PR is mergeable, %v", err)
		}

		if !prIsMergeable {
			log.Fatalf("PR is not mergeable")
		}

		err = ciService.MergePullRequest(prNumber)
		if err != nil {
			log.Fatalf("failed to merge PR, %v", err)
		}
	} else {
		log.Printf("PR is already merged, skipping merge step")
	}
}
