package configuration

import "digger/pkg/core/models"

type DiggerConfig struct {
	Projects         []ProjectConfig
	AutoMerge        bool
	CollectUsageData bool
	Workflows        map[string]WorkflowConfig
}

type ProjectConfig struct {
	Name            string
	Dir             string
	Workspace       string
	Terragrunt      bool
	Workflow        string
	IncludePatterns []string
	ExcludePatterns []string
}

type WorkflowConfig struct {
	EnvVars       *TerraformEnvConfig
	Plan          *models.Stage
	Apply         *models.Stage
	Configuration *WorkflowConfiguration
}

type WorkflowConfiguration struct {
	OnPullRequestPushed []string
	OnPullRequestClosed []string
	OnCommitToDefault   []string
}

type TerraformEnvConfig struct {
	State    []EnvVar
	Commands []EnvVar
}

type EnvVar struct {
	Name      string
	ValueFrom string
	Value     string
}

func defaultWorkflow() *WorkflowConfig {
	return &WorkflowConfig{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &models.Stage{
			Steps: []models.Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &models.Stage{
			Steps: []models.Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
	}
}
