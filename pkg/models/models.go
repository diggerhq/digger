package models

import "digger/pkg/configuration"

type ProjectCommand struct {
	ProjectName      string
	ProjectDir       string
	ProjectWorkspace string
	Terragrunt       bool
	Commands         []string
	ApplyStage       *configuration.Stage
	PlanStage        *configuration.Stage
	StateEnvVars     map[string]string
	CommandEnvVars   map[string]string
}
