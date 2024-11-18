package digger

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	coreutils "github.com/diggerhq/digger/libs/comment_utils/utils"
	"github.com/diggerhq/digger/libs/execution"
	locking2 "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/policy"
	orchestrator "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/storage"
	"log"
	"os"
	"path"
	"strings"
	"time"

	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	"github.com/diggerhq/digger/cli/pkg/usage"
	utils "github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	config "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/iac_utils"

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

func RunJobs(jobs []orchestrator.Job, prService ci.PullRequestService, orgService ci.OrgService, lock locking2.Lock, reporter reporting.Reporter, planStorage storage.PlanStorage, policyChecker policy.Checker, commentUpdater comment_updater.CommentUpdater, backendApi backendapi.Api, jobId string, reportFinalStatusToBackend bool, reportTerraformOutput bool, prCommentId string, workingDir string) (bool, bool, error) {

	defer reporter.Flush()

	runStartedAt := time.Now()

	exectorResults := make([]execution.DiggerExecutorResult, len(jobs))
	appliesPerProject := make(map[string]bool)

	for i, job := range jobs {
		splits := strings.Split(job.Namespace, "/")
		SCMOrganisation := splits[0]
		SCMrepository := splits[1]

		for _, command := range job.Commands {
			allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, job.PullRequestNumber, job.RequestedBy, []string{})

			if err != nil {
				return false, false, fmt.Errorf("error checking policy: %v", err)
			}

			if !allowedToPerformCommand {
				msg := reportPolicyError(job.ProjectName, command, job.RequestedBy, reporter)
				log.Printf("Skipping command ... %v for project %v", command, job.ProjectName)
				log.Println(msg)
				appliesPerProject[job.ProjectName] = false
				continue
			}

			executorResult, output, err := run(command, job, policyChecker, orgService, SCMOrganisation, SCMrepository, job.PullRequestNumber, job.RequestedBy, reporter, lock, prService, job.Namespace, workingDir, planStorage, appliesPerProject)
			if err != nil {
				log.Printf("error while running command %v for project %v: %v", command, job.ProjectName, err)
				reportErr := backendApi.ReportProjectRun(SCMOrganisation+"-"+SCMrepository, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, output)
				if reportErr != nil {
					log.Printf("error reporting project Run err: %v.\n", reportErr)
				}
				appliesPerProject[job.ProjectName] = false
				if executorResult != nil {
					exectorResults[i] = *executorResult
				}
				log.Printf("Project %v command %v failed, skipping job", job.ProjectName, command)
				break
			}
			exectorResults[i] = *executorResult

			err = backendApi.ReportProjectRun(SCMOrganisation+"-"+SCMrepository, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, output)
			if err != nil {
				log.Printf("Error reporting project Run: %v", err)
			}
		}
	}

	allAppliesSuccess := true
	for _, success := range appliesPerProject {
		if !success {
			allAppliesSuccess = false
		}
	}

	if allAppliesSuccess == true && reportFinalStatusToBackend == true {
		_, jobPrCommentUrl, err := reporter.Flush()
		if err != nil {
			log.Printf("error while sending job comments %v", err)
			return false, false, fmt.Errorf("error while sending job comments %v", err)
		}

		currentJob := jobs[0]
		repoNameForBackendReporting := strings.ReplaceAll(currentJob.Namespace, "/", "-")
		projectNameForBackendReporting := currentJob.ProjectName
		// TODO: handle the apply result summary as well to report it to backend. Possibly reporting changed resources as well
		// Some kind of generic terraform operation summary might need to be introduced
		summary := exectorResults[0].GetTerraformSummary()
		terraformOutput := ""
		if reportTerraformOutput {
			terraformOutput = exectorResults[0].TerraformOutput
		}
		prNumber := *currentJob.PullRequestNumber

		iacUtils := iac_utils.GetIacUtilsIacType(currentJob.IacType())
		batchResult, err := backendApi.ReportProjectJobStatus(repoNameForBackendReporting, projectNameForBackendReporting, jobId, "succeeded", time.Now(), &summary, "", jobPrCommentUrl, terraformOutput, iacUtils)
		if err != nil {
			log.Printf("error reporting Job status: %v.\n", err)
			return false, false, fmt.Errorf("error while running command: %v", err)
		}

		err = commentUpdater.UpdateComment(batchResult.Jobs, prNumber, prService, prCommentId)
		if err != nil {
			log.Printf("error Updating status comment: %v.\n", err)
			return false, false, err
		}
		err = UpdateAggregateStatus(batchResult, prService)
		if err != nil {
			log.Printf("error updating aggregate status check: %v.\n", err)
			return false, false, err
		}

	}

	atLeastOneApply := len(appliesPerProject) > 0

	return allAppliesSuccess, atLeastOneApply, nil
}

func reportPolicyError(projectName string, command string, requestedBy string, reporter reporting.Reporter) string {
	msg := fmt.Sprintf("User %s is not allowed to perform action: %s. Check your policies :x:", requestedBy, command)
	if reporter.SupportsMarkdown() {
		_, _, err := reporter.Report(msg, coreutils.AsCollapsibleComment(fmt.Sprintf("Policy violation for <b>%v - %v</b>", projectName, command), false))
		if err != nil {
			log.Printf("Error publishing comment: %v", err)
		}
	} else {
		_, _, err := reporter.Report(msg, coreutils.AsComment(fmt.Sprintf("Policy violation for %v - %v", projectName, command)))
		if err != nil {
			log.Printf("Error publishing comment: %v", err)
		}
	}
	return msg
}

func run(command string, job orchestrator.Job, policyChecker policy.Checker, orgService ci.OrgService, SCMOrganisation string, SCMrepository string, PRNumber *int, requestedBy string, reporter reporting.Reporter, lock locking2.Lock, prService ci.PullRequestService, projectNamespace string, workingDir string, planStorage storage.PlanStorage, appliesPerProject map[string]bool) (*execution.DiggerExecutorResult, string, error) {
	log.Printf("Running '%s' for project '%s' (workflow: %s)\n", command, job.ProjectName, job.ProjectWorkflow)

	allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, job.PullRequestNumber, requestedBy, []string{})

	if err != nil {
		return nil, "error checking policy", fmt.Errorf("error checking policy: %v", err)
	}

	if !allowedToPerformCommand {
		msg := reportPolicyError(job.ProjectName, command, requestedBy, reporter)
		log.Println(msg)
		return nil, msg, errors.New(msg)
	}

	err = job.PopulateAwsCredentialsEnvVarsForJob()
	if err != nil {
		log.Fatalf("failed to fetch AWS keys, %v", err)
	}

	projectLock := &locking2.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        prService,
		ProjectName:      job.ProjectName,
		ProjectNamespace: projectNamespace,
		PrNumber:         *job.PullRequestNumber,
	}

	var terraformExecutor execution.TerraformExecutor
	var iacUtils iac_utils.IacUtils
	projectPath := path.Join(workingDir, job.ProjectDir)
	if job.Terragrunt {
		terraformExecutor = execution.Terragrunt{WorkingDir: projectPath}
		iacUtils = iac_utils.TerraformUtils{}
	} else if job.OpenTofu {
		terraformExecutor = execution.OpenTofu{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
		iacUtils = iac_utils.TerraformUtils{}
	} else if job.Pulumi {
		terraformExecutor = execution.Pulumi{WorkingDir: projectPath, Stack: job.ProjectWorkspace}
		iacUtils = iac_utils.PulumiUtils{}
	} else {
		terraformExecutor = execution.Terraform{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
		iacUtils = iac_utils.TerraformUtils{}
	}

	commandRunner := execution.CommandRunner{}
	planPathProvider := execution.ProjectPathProvider{
		ProjectPath:      projectPath,
		ProjectNamespace: projectNamespace,
		ProjectName:      job.ProjectName,
		PRNumber:         PRNumber,
	}

	diggerExecutor := execution.LockingExecutorWrapper{
		ProjectLock: projectLock,
		Executor: execution.DiggerExecutor{
			ProjectNamespace:  projectNamespace,
			ProjectName:       job.ProjectName,
			ProjectPath:       projectPath,
			StateEnvVars:      job.StateEnvVars,
			RunEnvVars:        job.RunEnvVars,
			CommandEnvVars:    job.CommandEnvVars,
			ApplyStage:        job.ApplyStage,
			PlanStage:         job.PlanStage,
			CommandRunner:     commandRunner,
			TerraformExecutor: terraformExecutor,
			Reporter:          reporter,
			PlanStorage:       planStorage,
			PlanPathProvider:  planPathProvider,
			IacUtils:          iacUtils,
		},
	}
	executor := diggerExecutor.Executor.(execution.DiggerExecutor)

	switch command {

	case "digger plan":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "plan")
		if err != nil {
			log.Printf("failed to send usage report. %v", err)
		}
		err = prService.SetStatus(*job.PullRequestNumber, "pending", job.ProjectName+"/plan")
		if err != nil {
			msg := fmt.Sprintf("Failed to set PR status. %v", err)
			return nil, msg, fmt.Errorf(msg)
		}
		planSummary, planPerformed, isNonEmptyPlan, plan, planJsonOutput, err := diggerExecutor.Plan()

		if err != nil {
			msg := fmt.Sprintf("Failed to Run digger plan command. %v", err)
			log.Printf(msg)
			err := prService.SetStatus(*job.PullRequestNumber, "failure", job.ProjectName+"/plan")
			if err != nil {
				msg := fmt.Sprintf("Failed to set PR status. %v", err)
				return nil, msg, fmt.Errorf(msg)
			}

			return nil, msg, fmt.Errorf(msg)
		} else if planPerformed {
			if isNonEmptyPlan {
				reportTerraformPlanOutput(reporter, projectLock.LockId(), plan)
				planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, SCMOrganisation, job.ProjectName, job.ProjectDir, planJsonOutput)
				if err != nil {
					msg := fmt.Sprintf("Failed to validate plan. %v", err)
					log.Printf(msg)
					return nil, msg, fmt.Errorf(msg)
				}
				var planPolicyFormatter func(report string) string
				summary := fmt.Sprintf("Terraform plan validation check (%v)", job.ProjectName)
				if reporter.SupportsMarkdown() {
					planPolicyFormatter = coreutils.AsCollapsibleComment(summary, false)
				} else {
					planPolicyFormatter = coreutils.AsComment(summary)
				}

				planSummary, err := iacUtils.GetSummarizePlan(planJsonOutput)
				if err != nil {
					log.Printf("Failed to summarize plan. %v", err)
				}

				if !planIsAllowed {
					planReportMessage := "Terraform plan failed validation checks :x:<br>"
					preformattedMessaged := make([]string, 0)
					for _, message := range messages {
						preformattedMessaged = append(preformattedMessaged, fmt.Sprintf("    %v", message))
					}
					planReportMessage = planReportMessage + strings.Join(preformattedMessaged, "<br>")
					_, _, err = reporter.Report(planReportMessage, planPolicyFormatter)

					if err != nil {
						log.Printf("Failed to report plan. %v", err)
					}
					msg := fmt.Sprintf("Plan is not allowed")
					log.Printf(msg)
					return nil, msg, fmt.Errorf(msg)
				} else {
					_, _, err := reporter.Report("Terraform plan validation checks succeeded :white_check_mark:", planPolicyFormatter)
					if err != nil {
						log.Printf("Failed to report plan. %v", err)
					}
					reportPlanSummary(reporter, planSummary)
				}
			} else {
				reportEmptyPlanOutput(reporter, projectLock.LockId())
			}
			err := prService.SetStatus(*job.PullRequestNumber, "success", job.ProjectName+"/plan")
			if err != nil {
				msg := fmt.Sprintf("Failed to set PR status. %v", err)
				return nil, msg, fmt.Errorf(msg)
			}
			result := execution.DiggerExecutorResult{
				OperationType:   execution.DiggerOparationTypePlan,
				TerraformOutput: plan,
				PlanResult: &execution.DiggerExecutorPlanResult{
					PlanSummary:   *planSummary,
					TerraformJson: planJsonOutput,
				},
			}
			return &result, plan, nil
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
			return nil, msg, fmt.Errorf(msg)
		}

		isMerged, err := prService.IsMerged(*job.PullRequestNumber)
		if err != nil {
			msg := fmt.Sprintf("Failed to check if PR is merged. %v", err)
			return nil, msg, fmt.Errorf(msg)
		}

		// this might go into some sort of "appliability" plugin later
		isMergeable, err := prService.IsMergeable(*job.PullRequestNumber)
		if err != nil {
			msg := fmt.Sprintf("Failed to check if PR is mergeable. %v", err)
			return nil, msg, fmt.Errorf(msg)
		}
		log.Printf("PR status, mergeable: %v, merged: %v and skipMergeCheck %v\n", isMergeable, isMerged, job.SkipMergeCheck)
		if !isMergeable && !isMerged && !job.SkipMergeCheck {
			comment := reportApplyMergeabilityError(reporter)
			prService.SetStatus(*job.PullRequestNumber, "failure", job.ProjectName+"/apply")

			return nil, comment, fmt.Errorf(comment)
		} else {

			// checking policies (plan, access)
			var planPolicyViolations []string

			if os.Getenv("PLAN_UPLOAD_DESTINATION") != "" {
				terraformPlanJsonStr, err := executor.RetrievePlanJson()
				if err != nil {
					msg := fmt.Sprintf("Failed to retrieve stored plan. %v", err)
					log.Printf(msg)
					return nil, msg, fmt.Errorf(msg)
				}

				_, violations, err := policyChecker.CheckPlanPolicy(SCMrepository, SCMOrganisation, job.ProjectName, job.ProjectDir, terraformPlanJsonStr)
				if err != nil {
					msg := fmt.Sprintf("Failed to check plan policy. %v", err)
					log.Printf(msg)
					return nil, msg, fmt.Errorf(msg)
				}
				planPolicyViolations = violations
			} else {
				log.Printf("Skipping plan policy checks because plan storage is not configured.")
				planPolicyViolations = []string{}
			}

			allowedToApply, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, job.PullRequestNumber, requestedBy, planPolicyViolations)
			if err != nil {
				msg := fmt.Sprintf("Failed to run plan policy check before apply. %v", err)
				log.Printf(msg)
				return nil, msg, fmt.Errorf(msg)
			}
			if !allowedToApply {
				msg := reportPolicyError(job.ProjectName, command, requestedBy, reporter)
				log.Println(msg)
				return nil, msg, errors.New(msg)
			}

			// Running apply

			applySummary, applyPerformed, output, err := diggerExecutor.Apply()
			if err != nil {
				//TODO reuse executor error handling
				log.Printf("Failed to Run digger apply command. %v", err)
				err := prService.SetStatus(*job.PullRequestNumber, "failure", job.ProjectName+"/apply")
				if err != nil {
					msg := fmt.Sprintf("Failed to set PR status. %v", err)
					return nil, msg, fmt.Errorf(msg)
				}
				msg := fmt.Sprintf("Failed to run digger apply command. %v", err)
				return nil, msg, fmt.Errorf(msg)
			} else if applyPerformed {
				err := prService.SetStatus(*job.PullRequestNumber, "success", job.ProjectName+"/apply")
				if err != nil {
					msg := fmt.Sprintf("Failed to set PR status. %v", err)
					return nil, msg, fmt.Errorf(msg)
				}
				appliesPerProject[job.ProjectName] = true
			}
			result := execution.DiggerExecutorResult{
				OperationType:   execution.DiggerOparationTypeApply,
				TerraformOutput: output,
				ApplyResult: &execution.DiggerExecutorApplyResult{
					ApplySummary: *applySummary,
				},
			}
			return &result, output, nil
		}
	case "digger destroy":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "destroy")
		if err != nil {
			log.Printf("Failed to send usage report. %v", err)
		}
		_, err = diggerExecutor.Destroy()

		if err != nil {
			log.Printf("Failed to Run digger destroy command. %v", err)
			msg := fmt.Sprintf("failed to run digger destroy command: %v", err)
			return nil, msg, fmt.Errorf("failed to Run digger apply command. %v", err)
		}
		result := execution.DiggerExecutorResult{}
		return &result, "", nil

	case "digger unlock":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "unlock")
		if err != nil {
			log.Printf("failed to send usage report. %v", err)
		}
		err = diggerExecutor.Unlock()
		if err != nil {
			msg := fmt.Sprintf("Failed to unlock project. %v", err)
			return nil, msg, fmt.Errorf(msg)
		}

		if planStorage != nil {
			err = planStorage.DeleteStoredPlan(planPathProvider.ArtifactName(), planPathProvider.StoredPlanFilePath())
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
			return nil, msg, fmt.Errorf(msg)
		}

	default:
		msg := fmt.Sprintf("Command '%s' is not supported", command)
		return nil, msg, fmt.Errorf(msg)
	}
	return &execution.DiggerExecutorResult{}, "", nil
}

func reportApplyMergeabilityError(reporter reporting.Reporter) string {
	comment := "cannot perform Apply since the PR is not currently mergeable"
	log.Println(comment)

	if reporter.SupportsMarkdown() {
		_, _, err := reporter.Report(comment, coreutils.AsCollapsibleComment("Apply error", false))
		if err != nil {
			log.Printf("error publishing comment: %v\n", err)
		}
	} else {
		_, _, err := reporter.Report(comment, coreutils.AsComment("Apply error"))
		if err != nil {
			log.Printf("error publishing comment: %v\n", err)
		}
	}
	return comment
}

func reportTerraformPlanOutput(reporter reporting.Reporter, projectId string, plan string) {
	var formatter func(string) string

	if reporter.SupportsMarkdown() {
		formatter = coreutils.GetTerraformOutputAsCollapsibleComment("Plan output", true)
	} else {
		formatter = coreutils.GetTerraformOutputAsComment("Plan output")
	}

	_, _, err := reporter.Report(plan, formatter)
	if err != nil {
		log.Printf("Failed to report plan. %v", err)
	}
}

func reportPlanSummary(reporter reporting.Reporter, summary string) {
	var formatter func(string) string

	if reporter.SupportsMarkdown() {
		formatter = coreutils.AsCollapsibleComment("Plan summary", false)
	} else {
		formatter = coreutils.AsComment("Plan summary")
	}

	_, _, err := reporter.Report("\n"+summary, formatter)
	if err != nil {
		log.Printf("Failed to report plan summary. %v", err)
	}
}

func reportEmptyPlanOutput(reporter reporting.Reporter, projectId string) {
	identityFormatter := func(comment string) string {
		return comment
	}
	_, _, err := reporter.Report("→ No changes in terraform output for "+projectId, identityFormatter)
	// suppress the comment (if reporter is suppressible)
	reporter.Suppress()
	if err != nil {
		log.Printf("Failed to report plan. %v", err)
	}
}

func RunJob(
	job orchestrator.Job,
	repo string,
	requestedBy string,
	orgService ci.OrgService,
	policyChecker policy.Checker,
	planStorage storage.PlanStorage,
	backendApi backendapi.Api,
	driftNotification *core_drift.Notification,
	workingDir string,
) error {
	runStartedAt := time.Now()
	SCMOrganisation, SCMrepository := utils.ParseRepoNamespace(repo)
	log.Printf("Running '%s' for project '%s'\n", job.Commands, job.ProjectName)

	for _, command := range job.Commands {

		allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, nil, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, nil, requestedBy, []string{})

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
				log.Printf("Error reporting Run: %v", err)
			}
			return errors.New(msg)
		}

		err = job.PopulateAwsCredentialsEnvVarsForJob()
		if err != nil {
			log.Fatalf("failed to fetch AWS keys, %v", err)
		}

		var terraformExecutor execution.TerraformExecutor
		var iacUtils iac_utils.IacUtils
		projectPath := path.Join(workingDir, job.ProjectDir)
		if job.Terragrunt {
			terraformExecutor = execution.Terragrunt{WorkingDir: projectPath}
			iacUtils = iac_utils.TerraformUtils{}
		} else if job.OpenTofu {
			terraformExecutor = execution.OpenTofu{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
			iacUtils = iac_utils.TerraformUtils{}
		} else if job.Pulumi {
			terraformExecutor = execution.Pulumi{WorkingDir: projectPath, Stack: job.ProjectWorkspace}
			iacUtils = iac_utils.PulumiUtils{}
		} else {
			terraformExecutor = execution.Terraform{WorkingDir: projectPath, Workspace: job.ProjectWorkspace}
			iacUtils = iac_utils.TerraformUtils{}
		}

		commandRunner := execution.CommandRunner{}

		planPathProvider := execution.ProjectPathProvider{
			ProjectPath:      projectPath,
			ProjectNamespace: repo,
			ProjectName:      job.ProjectName,
			PRNumber:         job.PullRequestNumber,
		}

		diggerExecutor := execution.DiggerExecutor{
			ProjectNamespace:  repo,
			ProjectName:       job.ProjectName,
			ProjectPath:       projectPath,
			StateEnvVars:      job.StateEnvVars,
			RunEnvVars:        job.RunEnvVars,
			CommandEnvVars:    job.CommandEnvVars,
			ApplyStage:        job.ApplyStage,
			PlanStage:         job.PlanStage,
			CommandRunner:     commandRunner,
			Reporter:          &reporting.StdOutReporter{},
			TerraformExecutor: terraformExecutor,
			PlanStorage:       planStorage,
			PlanPathProvider:  planPathProvider,
			IacUtils:          iacUtils,
		}

		switch command {
		case "digger plan":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "plan")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, _, _, plan, planJsonOutput, err := diggerExecutor.Plan()
			if err != nil {
				msg := fmt.Sprintf("Failed to Run digger plan command. %v", err)
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting Run: %v", err)
				}
				return fmt.Errorf(msg)
			}
			planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, SCMOrganisation, job.ProjectName, job.ProjectDir, planJsonOutput)
			log.Print(strings.Join(messages, "\n"))
			if err != nil {
				msg := fmt.Sprintf("Failed to validate plan %v", err)
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting Run: %v", err)
				}
				return fmt.Errorf(msg)
			}
			if !planIsAllowed {
				msg := fmt.Sprintf("Plan is not allowed")
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting Run: %v", err)
				}
				return fmt.Errorf(msg)
			} else {
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, plan)
				if err != nil {
					log.Printf("Error reporting Run: %v", err)
				}
			}

		case "digger apply":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "apply")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, _, output, err := diggerExecutor.Apply()
			if err != nil {
				msg := fmt.Sprintf("Failed to Run digger apply command. %v", err)
				log.Printf(msg)
				err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "FAILED", command, msg)
				if err != nil {
					log.Printf("Error reporting Run: %v", err)
				}
				return fmt.Errorf(msg)
			}
			err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, output)
			if err != nil {
				log.Printf("Error reporting Run: %v", err)
			}
		case "digger destroy":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "destroy")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, err = diggerExecutor.Destroy()
			if err != nil {
				log.Printf("Failed to Run digger destroy command. %v", err)
				return fmt.Errorf("failed to Run digger apply command. %v", err)
			}

		case "digger drift-detect":
			output, err := runDriftDetection(policyChecker, SCMOrganisation, SCMrepository, job.ProjectName, requestedBy, job.EventName, diggerExecutor, driftNotification)
			if err != nil {
				return fmt.Errorf("failed to Run digger drift-detect command. %v", err)
			}
			err = backendApi.ReportProjectRun(repo, job.ProjectName, runStartedAt, time.Now(), "SUCCESS", command, output)
			if err != nil {
				log.Printf("Error reporting Run: %v", err)
			}
		}

	}
	return nil
}

func runDriftDetection(policyChecker policy.Checker, SCMOrganisation string, SCMrepository string, projectName string, requestedBy string, eventName string, diggerExecutor execution.Executor, notification *core_drift.Notification) (string, error) {
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
	_, planPerformed, nonEmptyPlan, plan, _, err := diggerExecutor.Plan()
	if err != nil {
		msg := fmt.Sprintf("failed to Run digger plan command. %v", err)
		log.Printf(msg)
		return msg, fmt.Errorf(msg)
	}

	if planPerformed && nonEmptyPlan {
		if notification == nil {
			log.Print("Warning: no notification configured, not sending any notifications")
			return plan, nil
		}
		err := (*notification).Send(projectName, plan)
		if err != nil {
			log.Printf("Error sending drift drift: %v", err)
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

func MergePullRequest(ciService ci.PullRequestService, prNumber int) {
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
