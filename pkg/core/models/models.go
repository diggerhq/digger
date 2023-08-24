package models

import configuration "github.com/diggerhq/lib-digger-config"

type Job struct {
	ProjectName       string
	ProjectDir        string
	ProjectWorkspace  string
	Terragrunt        bool
	Commands          []string
	ApplyStage        *configuration.Stage
	PlanStage         *configuration.Stage
	PullRequestNumber *int
	EventName         string
	RequestedBy       string
	Namespace         string
	StateEnvVars      map[string]string
	CommandEnvVars    map[string]string
}
