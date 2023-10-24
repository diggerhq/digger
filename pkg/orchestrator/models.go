package orchestrator

import configuration "digger/pkg/digger_config"

type Job struct {
	ProjectName       string
	ProjectDir        string
	ProjectWorkspace  string
	ProjectWorkflow   string
	Terragrunt        bool
	Commands          []string
	ApplyStage        *Stage
	PlanStage         *Stage
	PullRequestNumber *int
	EventName         string
	RequestedBy       string
	Namespace         string
	StateEnvVars      map[string]string
	CommandEnvVars    map[string]string
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
