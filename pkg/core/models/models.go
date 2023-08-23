package models

type Job struct {
	ProjectName       string            `json:"projectName"`
	ProjectDir        string            `json:"projectDir"`
	ProjectWorkspace  string            `json:"projectWorkspace"`
	Terragrunt        bool              `json:"terragrunt"`
	Commands          []string          `json:"commands"`
	ApplyStage        *Stage            `json:"applyStage"`
	PlanStage         *Stage            `json:"planStage"`
	PullRequestNumber *int              `json:"pullRequestNumber"`
	EventName         string            `json:"eventName"`
	RequestedBy       string            `json:"requestedBy"`
	Namespace         string            `json:"namespace"`
	StateEnvVars      map[string]string `json:"stateEnvVars"`
	CommandEnvVars    map[string]string `json:"commandEnvVars"`
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
