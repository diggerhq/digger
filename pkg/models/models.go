package models

import "digger/pkg/configuration"

type ProjectCommand struct {
	ProjectName      string
	ProjectDir       string
	ProjectWorkspace string
	Terragrunt       bool
	Commands         []string
	ApplyStage       *configuration.StageConfig
	PlanStage        *configuration.StageConfig
	StateEnvVars     map[string]string
	CommandEnvVars   map[string]string
}
