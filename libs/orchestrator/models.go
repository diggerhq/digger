package orchestrator

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/diggerhq/digger/libs/digger_config"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"log"
	"slices"
)

type Job struct {
	ProjectName        string
	ProjectDir         string
	ProjectWorkspace   string
	ProjectWorkflow    string
	Terragrunt         bool
	OpenTofu           bool
	Commands           []string
	ApplyStage         *Stage
	PlanStage          *Stage
	PullRequestNumber  *int
	EventName          string
	RequestedBy        string
	Namespace          string
	RunEnvVars         map[string]string
	StateEnvVars       map[string]string
	CommandEnvVars     map[string]string
	StateEnvProvider   *stscreds.WebIdentityRoleProvider
	CommandEnvProvider *stscreds.WebIdentityRoleProvider
}

type Step struct {
	Action    string
	Value     string
	ExtraArgs []string
	Shell     string
}

type Stage struct {
	Steps []Step
}

func ToConfigStep(configState configuration.Step) Step {
	return Step{
		Action:    configState.Action,
		Value:     configState.Value,
		ExtraArgs: configState.ExtraArgs,
		Shell:     configState.Shell,
	}

}

func ToConfigStage(configStage *configuration.Stage) *Stage {
	if configStage == nil {
		return nil
	}
	steps := make([]Step, 0)
	for _, step := range configStage.Steps {
		steps = append(steps, ToConfigStep(step))
	}
	return &Stage{
		Steps: steps,
	}
}

func (j *Job) IsPlan() bool {
	return slices.Contains(j.Commands, "digger plan")
}

func (j *Job) IsApply() bool {
	return slices.Contains(j.Commands, "digger apply")
}

func IsPlanJobs(jobs []Job) bool {
	isPlan := true
	for _, job := range jobs {
		isPlan = isPlan && job.IsPlan()
	}
	return isPlan
}

func IsApplyJobs(jobs []JobJson) bool {
	isApply := true
	for _, job := range jobs {
		isApply = isApply && job.IsApply()
	}
	return isApply
}

func ConvertProjectsToJobs(actor string, repoNamespace string, command string, prNumber int, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]Job, bool, error) {
	jobs := make([]Job, 0)

	log.Printf("ConvertToCommands, command: %s\n", command)
	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			return nil, true, fmt.Errorf("failed to find workflow digger_config '%s' for project '%s'", project.Workflow, project.Name)
		}

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)
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
		})
	}
	return jobs, true, nil
}
