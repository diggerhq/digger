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
			StateRoleArn:     	stateRole,	
			CommandEnvProvider: CommandEnvProvider,
			CommandRoleArn:     cmdRole,
			CognitoOidcConfig:  project.AwsCognitoOidcConfig,
		})
	}
	return jobs, true, nil
}
