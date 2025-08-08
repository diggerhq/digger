package digger

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	coreutils "github.com/diggerhq/digger/libs/comment_utils/utils"
	"github.com/diggerhq/digger/libs/execution"
	locking2 "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/policy"
	orchestrator "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/storage"

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

	slog.Debug("Variable info", "TF_PLUGIN_CACHE_DIR", os.Getenv("TF_PLUGIN_CACHE_DIR"))
	slog.Debug("Variable info", "TG_PROVIDER_CACHE_DIR", os.Getenv("TG_PROVIDER_CACHE_DIR"))
	slog.Debug("Variable info", "TERRAGRUNT_PROVIDER_CACHE_DIR", os.Getenv("TERRAGRUNT_PROVIDER_CACHE_DIR"))

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
				slog.Warn("Skipping command ... %v for project %v", command, job.ProjectName)
				slog.Warn("Received policy error", "message", msg)
				appliesPerProject[job.ProjectName] = false
				continue
			}

			executorResult, _, err := run(command, job, policyChecker, orgService, SCMOrganisation, SCMrepository, job.PullRequestNumber, job.RequestedBy, reporter, lock, prService, job.Namespace, workingDir, planStorage, appliesPerProject)
			if err != nil {
				slog.Error("error while running command for project", "command", command, "projectname", job.ProjectName, "error", err)
				appliesPerProject[job.ProjectName] = false
				if executorResult != nil {
					exectorResults[i] = *executorResult
				}
				slog.Error("Project command failed, skipping job", "project name", job.ProjectName, "command", command)
				break
			}
			exectorResults[i] = *executorResult

		}
	}

	allAppliesSuccess := true
	for _, success := range appliesPerProject {
		if !success {
			allAppliesSuccess = false
		}
	}

	if allAppliesSuccess == true && reportFinalStatusToBackend == true {
		currentJob := jobs[0]

		jobPrCommentId, jobPrCommentUrl, err := reporter.Flush()
		if err != nil {
			slog.Error("error while sending job comments", "error", err)
			cmt, cmt_err := prService.PublishComment(*currentJob.PullRequestNumber, fmt.Sprintf(":yellow_circle: Warning: failed to post report for project %v, received error: %v.\n\n you may review details in the job logs", currentJob.ProjectName, err))
			if cmt_err != nil {
				slog.Error("Error while posting error comment", "error", err)
				return false, false, fmt.Errorf("failed to post reporter error comment, aborting. Error: %v", err)
			}
			jobPrCommentUrl = cmt.Url
		}

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
		batchResult, err := backendApi.ReportProjectJobStatus(currentJob.Namespace, projectNameForBackendReporting, jobId, "succeeded", time.Now(), &summary, "", jobPrCommentUrl, jobPrCommentId, terraformOutput, iacUtils)
		if err != nil {
			slog.Error("error reporting Job status", "error", err)
			return false, false, fmt.Errorf("error while running command: %v", err)
		}

		err = commentUpdater.UpdateComment(batchResult.Jobs, prNumber, prService, prCommentId)
		if err != nil {
			slog.Error("error Updating status comment", "error", err)
			return false, false, err
		}
		err = UpdateAggregateStatus(batchResult, prService)
		if err != nil {
			slog.Error("error updating aggregate status check", "error", err)
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
			slog.Error("Error publishing comment", "error", err)
		}
	} else {
		_, _, err := reporter.Report(msg, coreutils.AsComment(fmt.Sprintf("Policy violation for %v - %v", projectName, command)))
		if err != nil {
			slog.Error("Error publishing comment", "error", err)
		}
	}
	return msg
}

func run(command string, job orchestrator.Job, policyChecker policy.Checker, orgService ci.OrgService, SCMOrganisation string, SCMrepository string, PRNumber *int, requestedBy string, reporter reporting.Reporter, lock locking2.Lock, prService ci.PullRequestService, projectNamespace string, workingDir string, planStorage storage.PlanStorage, appliesPerProject map[string]bool) (*execution.DiggerExecutorResult, string, error) {
	slog.Info("Running command for project", "command", command, "project name", job.ProjectName, "project workflow", job.ProjectWorkflow)

	allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, job.PullRequestNumber, requestedBy, []string{})

	if err != nil {
		return nil, "error checking policy", fmt.Errorf("error checking policy: %v", err)
	}

	if !allowedToPerformCommand {
		msg := reportPolicyError(job.ProjectName, command, requestedBy, reporter)
		slog.Error(msg)
		return nil, msg, errors.New(msg)
	}

	err = job.PopulateAwsCredentialsEnvVarsForJob()
	if err != nil {
		slog.Error("failed to fetch AWS keys", "error", err)
		os.Exit(1)
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
			slog.Error("failed to send usage report", "error", err)
		}
		err = prService.SetStatus(*job.PullRequestNumber, "pending", job.GetProjectAlias()+"/plan")
		if err != nil {
			msg := fmt.Sprintf("Failed to set PR status. %v", err)
			return nil, msg, fmt.Errorf("%s", msg)
		}
		planSummary, planPerformed, isNonEmptyPlan, plan, planJsonOutput, err := diggerExecutor.Plan()

		if err != nil {
			msg := fmt.Sprintf("Failed to Run digger plan command. %v", err)
			slog.Error("Failed to Run digger plan command", "error", err)
			err := prService.SetStatus(*job.PullRequestNumber, "failure", job.GetProjectAlias()+"/plan")
			if err != nil {
				msg := fmt.Sprintf("Failed to set PR status. %v", err)
				return nil, msg, fmt.Errorf("%s", msg)
			}

			return nil, msg, fmt.Errorf("%s", msg)
		} else if planPerformed {
			if isNonEmptyPlan {
				reportTerraformPlanOutput(reporter, projectLock.LockId(), plan)
				planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, SCMOrganisation, job.ProjectName, job.ProjectDir, planJsonOutput)
				if err != nil {
					msg := fmt.Sprintf("Failed to validate plan. %v", err)
					slog.Error("Failed to validate plan.", "error", err)
					return nil, msg, fmt.Errorf("%s", msg)
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
					slog.Error("Failed to summarize plan", "error", err)
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
						slog.Error("Failed to report plan.", "error", err)
					}
					msg := fmt.Sprintf("Plan is not allowed")
					slog.Error(msg)
					return nil, msg, fmt.Errorf("%s", msg)
				} else {
					_, _, err := reporter.Report("Terraform plan validation checks succeeded :white_check_mark:", planPolicyFormatter)
					if err != nil {
						slog.Error("Failed to report plan.", "error", err)
					}
					reportPlanSummary(reporter, planSummary)
				}
			} else {
				reportEmptyPlanOutput(reporter, projectLock.LockId())
			}
			err := prService.SetStatus(*job.PullRequestNumber, "success", job.GetProjectAlias()+"/plan")
			if err != nil {
				msg := fmt.Sprintf("Failed to set PR status. %v", err)
				return nil, msg, fmt.Errorf("%s", msg)
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
			slog.Error("failed to send usage report.", "error", err)
		}
		err = prService.SetStatus(*job.PullRequestNumber, "pending", job.GetProjectAlias()+"/apply")
		if err != nil {
			msg := fmt.Sprintf("Failed to set PR status. %v", err)
			return nil, msg, fmt.Errorf("%s", msg)
		}

		isMerged, err := prService.IsMerged(*job.PullRequestNumber)
		if err != nil {
			msg := fmt.Sprintf("Failed to check if PR is merged. %v", err)
			return nil, msg, fmt.Errorf("%s", msg)
		}

		// this might go into some sort of "appliability" plugin later
		isMergeable, err := prService.IsMergeable(*job.PullRequestNumber)
		if err != nil {
			msg := fmt.Sprintf("Failed to check if PR is mergeable. %v", err)
			return nil, msg, fmt.Errorf("%s", msg)
		}
		slog.Info("PR status Information", "mergeable", isMergeable, "merged", isMerged, "skipMergeCheck", job.SkipMergeCheck)
		if !isMergeable && !isMerged && !job.SkipMergeCheck {
			comment := reportApplyMergeabilityError(reporter)
			prService.SetStatus(*job.PullRequestNumber, "failure", job.GetProjectAlias()+"/apply")

			return nil, comment, fmt.Errorf("%s", comment)
		} else {

			// checking policies (plan, access)
			var planPolicyViolations []string

			if os.Getenv("PLAN_UPLOAD_DESTINATION") != "" {
				terraformPlanJsonStr, err := executor.RetrievePlanJson()
				if err != nil {
					msg := fmt.Sprintf("Failed to retrieve stored plan. %v", err)
					slog.Error("Failed to retrieve stored plan.", "error", err)
					return nil, msg, fmt.Errorf("%s", msg)
				}

				_, violations, err := policyChecker.CheckPlanPolicy(SCMrepository, SCMOrganisation, job.ProjectName, job.ProjectDir, terraformPlanJsonStr)
				if err != nil {
					msg := fmt.Sprintf("Failed to check plan policy. %v", err)
					slog.Error("Failed to check plan policy.", "error", err)
					return nil, msg, fmt.Errorf("%s", msg)
				}
				planPolicyViolations = violations
			} else {
				slog.Info("Skipping plan policy checks because plan storage is not configured.")
				planPolicyViolations = []string{}
			}

			allowedToApply, err := policyChecker.CheckAccessPolicy(orgService, &prService, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, job.PullRequestNumber, requestedBy, planPolicyViolations)
			if err != nil {
				msg := fmt.Sprintf("Failed to run plan policy check before apply. %v", err)
				slog.Error("Failed to run plan policy check before apply", "error", err)
				return nil, msg, fmt.Errorf("%s", msg)
			}
			if !allowedToApply {
				msg := reportPolicyError(job.ProjectName, command, requestedBy, reporter)
				slog.Error(msg)
				return nil, msg, errors.New(msg)
			}

			// Running apply

			applySummary, applyPerformed, output, err := diggerExecutor.Apply()
			if err != nil {
				//TODO reuse executor error handling
				slog.Error("Failed to Run digger apply command.", "error", err)
				err := prService.SetStatus(*job.PullRequestNumber, "failure", job.GetProjectAlias()+"/apply")
				if err != nil {
					msg := fmt.Sprintf("Failed to set PR status. %v", err)
					return nil, msg, fmt.Errorf("%s", msg)
				}
				msg := fmt.Sprintf("Failed to run digger apply command. %v", err)
				return nil, msg, fmt.Errorf("%s", msg)
			} else if applyPerformed {
				err := prService.SetStatus(*job.PullRequestNumber, "success", job.GetProjectAlias()+"/apply")
				if err != nil {
					msg := fmt.Sprintf("Failed to set PR status. %v", err)
					return nil, msg, fmt.Errorf("%s", msg)
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
			slog.Error("Failed to send usage report.", "error", err)
		}
		_, err = diggerExecutor.Destroy()

		if err != nil {
			slog.Error("Failed to Run digger destroy command.", "error", err)
			msg := fmt.Sprintf("failed to run digger destroy command: %v", err)
			return nil, msg, fmt.Errorf("failed to Run digger apply command. %v", err)
		}
		result := execution.DiggerExecutorResult{}
		return &result, "", nil

	case "digger unlock":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "unlock")
		if err != nil {
			slog.Error("failed to send usage report.", "error", err)
		}
		err = diggerExecutor.Unlock()
		if err != nil {
			msg := fmt.Sprintf("Failed to unlock project. %v", err)
			return nil, msg, fmt.Errorf("%s", msg)
		}

		if planStorage != nil {
			err = planStorage.DeleteStoredPlan(planPathProvider.ArtifactName(), planPathProvider.StoredPlanFilePath())
			if err != nil {
				slog.Error("failed to delete stored plan file", "stored plan file path", planPathProvider.StoredPlanFilePath(), "error", err)
			}
		}
	case "digger lock":
		err := usage.SendUsageRecord(requestedBy, job.EventName, "lock")
		if err != nil {
			slog.Error("failed to send usage report.", "error", err)
		}
		err = diggerExecutor.Lock()
		if err != nil {
			msg := fmt.Sprintf("Failed to lock project. %v", err)
			return nil, msg, fmt.Errorf("%s", msg)
		}

	default:
		msg := fmt.Sprintf("Command '%s' is not supported", command)
		return nil, msg, fmt.Errorf("%s", msg)
	}
	return &execution.DiggerExecutorResult{}, "", nil
}

func reportApplyMergeabilityError(reporter reporting.Reporter) string {
	comment := "cannot perform Apply since the PR is not currently mergeable"
	slog.Error(comment)

	if reporter.SupportsMarkdown() {
		_, _, err := reporter.Report(comment, coreutils.AsCollapsibleComment("Apply error", false))
		if err != nil {
			slog.Error("error publishing comment", "error", err)
		}
	} else {
		_, _, err := reporter.Report(comment, coreutils.AsComment("Apply error"))
		if err != nil {
			slog.Error("error publishing comment", "error", err)
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
		slog.Error("Failed to report plan.", "error", err)
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
		slog.Error("Failed to report plan summary.", "error", err)
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
		slog.Error("Failed to report plan.", "error", err)
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
	SCMOrganisation, SCMrepository := utils.ParseRepoNamespace(repo)
	slog.Info("Running commands for project", "commands", job.Commands, "project name", job.ProjectName)

	for _, command := range job.Commands {

		allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, nil, SCMOrganisation, SCMrepository, job.ProjectName, job.ProjectDir, command, nil, requestedBy, []string{})

		if err != nil {
			return fmt.Errorf("error checking policy: %v", err)
		}

		if !allowedToPerformCommand {
			msg := fmt.Sprintf("User %s is not allowed to perform action: %s. Check your policies", requestedBy, command)
			if err != nil {
				slog.Error("Error publishing comment.", "error", err)
			}
			slog.Error(msg)
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
				slog.Error("Failed to send usage report.", "error", err)
			}
			_, _, _, _, planJsonOutput, err := diggerExecutor.Plan()
			if err != nil {
				msg := fmt.Sprintf("Failed to Run digger plan command. %v", err)
				slog.Error(msg)
				return fmt.Errorf("%s", msg)
			}
			planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, SCMOrganisation, job.ProjectName, job.ProjectDir, planJsonOutput)
			slog.Info(strings.Join(messages, "\n"))
			if err != nil {
				msg := fmt.Sprintf("Failed to validate plan %v", err)
				slog.Error(msg)
				return fmt.Errorf("%s", msg)
			}
			if !planIsAllowed {
				msg := fmt.Sprintf("Plan is not allowed")
				slog.Error(msg)
				return fmt.Errorf("%s", msg)
			} else {
			}

		case "digger apply":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "apply")
			if err != nil {
				slog.Error("Failed to send usage report.", "error", err)
			}
			_, _, _, err = diggerExecutor.Apply()
			if err != nil {
				msg := fmt.Sprintf("Failed to Run digger apply command. %v", err)
				slog.Error(msg)
				return fmt.Errorf("%s", msg)
			}
		case "digger destroy":
			err := usage.SendUsageRecord(requestedBy, job.EventName, "destroy")
			if err != nil {
				slog.Error("Failed to send usage report.", "error", err)
			}
			_, err = diggerExecutor.Destroy()
			if err != nil {
				slog.Error("Failed to Run digger destroy command.", "error", err)
				return fmt.Errorf("failed to Run digger apply command. %v", err)
			}

		case "digger drift-detect":
			_, err = runDriftDetection(policyChecker, SCMOrganisation, SCMrepository, job.ProjectName, requestedBy, job.EventName, diggerExecutor, driftNotification)
			if err != nil {
				return fmt.Errorf("failed to Run digger drift-detect command. %v", err)
			}
		}

	}
	return nil
}

func runDriftDetection(policyChecker policy.Checker, SCMOrganisation string, SCMrepository string, projectName string, requestedBy string, eventName string, diggerExecutor execution.Executor, notification *core_drift.Notification) (string, error) {
	err := usage.SendUsageRecord(requestedBy, eventName, "drift-detect")
	if err != nil {
		slog.Error("Failed to send usage report.", "error", err)
	}
	policyEnabled, err := policyChecker.CheckDriftPolicy(SCMOrganisation, SCMrepository, projectName)
	if err != nil {
		msg := fmt.Sprintf("failed to check drift policy. %v", err)
		slog.Error(msg)
		return msg, fmt.Errorf("%s", msg)
	}

	if !policyEnabled {
		msg := "skipping this drift application since it is not enabled for this project"
		slog.Info(msg)
		return msg, nil
	}
	_, planPerformed, nonEmptyPlan, plan, _, err := diggerExecutor.Plan()
	if err != nil {
		msg := fmt.Sprintf("failed to Run digger plan command. %v", err)
		slog.Error(msg)
		return msg, fmt.Errorf("%s", msg)
	}

	if planPerformed && nonEmptyPlan {
		if notification == nil {
			slog.Warn("Warning: no notification configured, not sending any notifications")
			return plan, nil
		}
		repoFullName := fmt.Sprintf("%s/%s", SCMOrganisation, SCMrepository)
		err := (*notification).SendNotificationForProject(projectName, repoFullName, plan)
		if err != nil {
			slog.Error("Error sending drift drift.", "error", err)
			return plan, fmt.Errorf("failed to send drift. %v", err)
		}
	} else if planPerformed && !nonEmptyPlan {
		slog.Info("No drift detected")
	} else {
		slog.Info("No plan performed")
	}
	return plan, nil
}

func SortedCommandsByDependency(project []orchestrator.Job, dependencyGraph *graph.Graph[string, config.Project]) []orchestrator.Job {
	var sortedCommands []orchestrator.Job
	sortedGraph, err := graph.StableTopologicalSort(*dependencyGraph, func(s string, s2 string) bool {
		return s < s2
	})
	if err != nil {
		slog.Error("failed to sort commands by dependency", "error", err)
		slog.Debug("Dependency Graph Debug", "DependencyGraph", dependencyGraph)
		os.Exit(1)
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

func MergePullRequest(ciService ci.PullRequestService, prNumber int, mergeStrategy config.AutomergeStrategy) {
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

		err = ciService.MergePullRequest(prNumber, string(mergeStrategy))
		if err != nil {
			log.Fatalf("failed to merge PR, %v", err)
		}
	} else {
		slog.Info("PR is already merged, skipping merge step")
	}
}
