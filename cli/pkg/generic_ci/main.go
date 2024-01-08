package generic_ci

import (
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	"log"
)

func ConvertToCommands(actor string, repoNamespace string, command string, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]orchestrator.Job, bool, error) {
	jobs := make([]orchestrator.Job, 0)

	log.Printf("ConvertToCommands, command: %s\n", command)
	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			return nil, true, fmt.Errorf("failed to find workflow digger_config '%s' for project '%s'", project.Workflow, project.Name)
		}

		prNum := 1
		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)
		StateEnvProvider, CommandEnvProvider := orchestrator.GetStateAndCommandProviders(project)
		jobs = append(jobs, orchestrator.Job{
			ProjectName:      project.Name,
			ProjectDir:       project.Dir,
			ProjectWorkspace: project.Workspace,
			Terragrunt:       project.Terragrunt,
			OpenTofu:         project.OpenTofu,
			// TODO: expose lower level api per command configuration
			Commands:   []string{command},
			ApplyStage: orchestrator.ToConfigStage(workflow.Apply),
			PlanStage:  orchestrator.ToConfigStage(workflow.Plan),
			// TODO:
			PullRequestNumber:  &prNum,
			EventName:          "manual_run",
			RequestedBy:        actor,
			Namespace:          repoNamespace,
			StateEnvVars:       stateEnvVars,
			CommandEnvVars:     commandEnvVars,
			StateEnvProvider:   StateEnvProvider,
			CommandEnvProvider: CommandEnvProvider,
		})
	}
	return jobs, true, nil
}
