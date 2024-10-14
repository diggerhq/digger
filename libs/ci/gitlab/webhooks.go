package gitlab

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/dominikbraun/graph"
	"github.com/xanzy/go-gitlab"
)

func ProcessGitlabPullRequestEvent(payload *gitlab.MergeEvent, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService ci.PullRequestService) ([]digger_config.Project, map[string]digger_config.ProjectToSourceMapping, int, error) {
	var impactedProjects []digger_config.Project
	var prNumber int
	prNumber = payload.ObjectAttributes.IID
	changedFiles, err := ciService.GetChangedFiles(prNumber)

	if err != nil {
		return nil, nil, prNumber, fmt.Errorf("could not get changed files")
	}
	impactedProjects, impactedProjectsSourceLocations := diggerConfig.GetModifiedProjects(changedFiles)

	if diggerConfig.DependencyConfiguration.Mode == digger_config.DependencyConfigurationHard {
		impactedProjects, err = generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
		if err != nil {
			return nil, nil, prNumber, fmt.Errorf("failed to find all projects dependant on impacted projects")
		}
	}

	return impactedProjects, impactedProjectsSourceLocations, prNumber, nil
}

func ConvertGithubPullRequestEventToJobs(payload *gitlab.MergeEvent, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, config digger_config.DiggerConfig) ([]scheduler.Job, bool, error) {
	workflows := config.Workflows
	jobs := make([]scheduler.Job, 0)

	defaultBranch := payload.Repository.DefaultBranch
	prBranch := payload.ObjectAttributes.SourceBranch

	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
		}

		runEnvVars := generic.GetRunEnvVars(defaultBranch, prBranch, project.Name, project.Dir)

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, false)
		pullRequestNumber := payload.ObjectAttributes.IID
		namespace := payload.Project.PathWithNamespace
		sender := payload.User.Username


		var skipMerge bool
		if workflow.Configuration != nil {
			skipMerge = workflow.Configuration.SkipMergeCheck
		} else {
			skipMerge = false
		}

		StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)
		if payload.ObjectAttributes.Action == "merge" && payload.ObjectAttributes.TargetBranch == defaultBranch {
			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				Commands:           workflow.Configuration.OnCommitToDefault,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  &pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          namespace,
				RequestedBy:        sender,
				CommandEnvProvider: CommandEnvProvider,
				StateEnvProvider:   StateEnvProvider,
				SkipMergeCheck:     skipMerge,
			})
		} else if payload.ObjectAttributes.Action == "open" || payload.ObjectAttributes.Action == "reopen" || payload.ObjectAttributes.Action == "synchronize" {
			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Commands:           workflow.Configuration.OnPullRequestPushed,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  &pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          namespace,
				RequestedBy:        sender,
				CommandEnvProvider: CommandEnvProvider,
				StateEnvProvider:   StateEnvProvider,
				SkipMergeCheck:    	skipMerge,
			})
		} else if payload.ObjectAttributes.Action == "close" {
			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Commands:           workflow.Configuration.OnPullRequestClosed,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  &pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          namespace,
				RequestedBy:        sender,
				CommandEnvProvider: CommandEnvProvider,
				StateEnvProvider:   StateEnvProvider,
				SkipMergeCheck:    	skipMerge,
			})
			//	TODO: Figure how to detect gitlab's "PR converted to draft" event
		} else if payload.ObjectAttributes.Action == "converted_to_draft" {
			var commands []string
			if config.AllowDraftPRs == false && len(workflow.Configuration.OnPullRequestConvertedToDraft) == 0 {
				commands = []string{"digger unlock"}
			} else {
				commands = workflow.Configuration.OnPullRequestConvertedToDraft
			}

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Commands:           commands,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  &pullRequestNumber,
				EventName:          "pull_request_converted_to_draft",
				Namespace:          namespace,
				RequestedBy:        sender,
				CommandEnvProvider: CommandEnvProvider,
				StateEnvProvider:   StateEnvProvider,
				SkipMergeCheck:   	skipMerge,
			})
		}

	}
	return jobs, true, nil
}
