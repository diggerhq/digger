package configuration

import "digger/pkg/core/models"

type DiggerConfig struct {
	Projects         []Project
	AutoMerge        bool
	CollectUsageData bool
	Workflows        map[string]Workflow
}

type Project struct {
	Name               string
	Dir                string
	Workspace          string
	Terragrunt         bool
	Workflow           string
	IncludePatterns    []string
	ExcludePatterns    []string
	DependencyProjects []string
}

type Workflow struct {
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

func defaultWorkflow() *Workflow {
	return &Workflow{
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
		EnvVars: &TerraformEnvConfig{},
	}
}
