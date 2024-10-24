package scheduler

import (
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config"
	"log"
)

func ConvertProjectsToJobs(actor string, repoNamespace string, command string, prNumber int, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]Job, bool, error) {
	jobs := make([]Job, 0)

	log.Printf("ConvertToCommands, command: %s\n", command)
	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
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
		jobs = append(jobs, Job{
			ProjectName:      project.Name,
			ProjectDir:       project.Dir,
			ProjectWorkspace: project.Workspace,
			Terragrunt:       project.Terragrunt,
			OpenTofu:         project.OpenTofu,
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
			CommandEnvProvider: CommandEnvProvider,
			SkipMergeCheck: 	skipMerge,
		})
	}
	return jobs, true, nil
}
