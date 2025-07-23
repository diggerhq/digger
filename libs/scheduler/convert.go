package scheduler

import (
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/libs/digger_config"
)

func ConvertProjectsToJobs(actor, repoNamespace, command string, prNumber int, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]Job, bool, error) {
	jobs := make([]Job, 0)

	slog.Info("Converting projects to jobs",
		"command", command,
		"projectCount", len(impactedProjects))

	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			slog.Error("Failed to find workflow config",
				"workflowName", project.Workflow,
				"projectName", project.Name)
			return nil, true, fmt.Errorf("failed to find workflow digger_config '%s' for project '%s'", project.Workflow, project.Name)
		}

		var skipMerge bool
		if workflow.Configuration != nil {
			skipMerge = workflow.Configuration.SkipMergeCheck
		} else {
			skipMerge = false
		}

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, false)
		StateEnvProvider, CommandEnvProvider := GetStateAndCommandProviders(project)

		stateRole, cmdRole := "", ""

		if project.AwsRoleToAssume != nil {
			if project.AwsRoleToAssume.State != "" {
				stateRole = project.AwsRoleToAssume.State
			}

			if project.AwsRoleToAssume.Command != "" {
				cmdRole = project.AwsRoleToAssume.Command
			}
		}

		slog.Debug("Creating job for project",
			"projectName", project.Name,
			"projectDir", project.Dir,
			"workspace", project.Workspace,
			"terragrunt", project.Terragrunt,
			"openTofu", project.OpenTofu,
			"pulumi", project.Pulumi,
			"hasStateRole", stateRole != "",
			"hasCommandRole", cmdRole != "",
			"hasCognitoConfig", project.AwsCognitoOidcConfig != nil)

		jobs = append(jobs, Job{
			ProjectName:      project.Name,
			ProjectDir:       project.Dir,
			ProjectWorkspace: project.Workspace,
			Terragrunt:       project.Terragrunt,
			OpenTofu:         project.OpenTofu,
			Pulumi:           project.Pulumi,
			// TODO: expose lower level api per command configuration
			Commands:   []string{command},
			ApplyStage: ToConfigStage(workflow.Apply),
			PlanStage:  ToConfigStage(workflow.Plan),
			// TODO:
			PullRequestNumber:  &prNumber,
			EventName:          "manual_run",
			RequestedBy:        actor,
			Namespace:          repoNamespace,
			StateEnvVars:       stateEnvVars,
			CommandEnvVars:     commandEnvVars,
			StateEnvProvider:   StateEnvProvider,
			StateRoleArn:       stateRole,
			CommandEnvProvider: CommandEnvProvider,
			CommandRoleArn:     cmdRole,
			CognitoOidcConfig:  project.AwsCognitoOidcConfig,
			SkipMergeCheck:     skipMerge,
		})
	}

	slog.Info("Successfully converted projects to jobs", "jobCount", len(jobs))
	return jobs, true, nil
}
