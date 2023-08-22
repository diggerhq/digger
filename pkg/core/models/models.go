package models

type Job struct {
	ProjectName       string
	ProjectDir        string
	ProjectWorkspace  string
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
