package digger_config

type DiggerConfig struct {
	DependencyConfiguration DependencyConfiguration
	Projects                []Project
	AutoMerge               bool
	Telemetry               bool
	Workflows               map[string]Workflow
}

type DependencyConfiguration struct {
	Mode string
}

type AssumeRoleForProject struct {
	State   string
	Command string
}

type Project struct {
	Name               string
	Dir                string
	Workspace          string
	Terragrunt         bool
	OpenTofu           bool
	Workflow           string
	IncludePatterns    []string
	ExcludePatterns    []string
	DependencyProjects []string
	DriftDetection     bool
	AwsRoleToAssume    *AssumeRoleForProject
}

type Workflow struct {
	EnvVars       *TerraformEnvConfig
	Plan          *Stage
	Apply         *Stage
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

type Step struct {
	Action    string
	Value     string
	ExtraArgs []string
	Shell     string
}

type Stage struct {
	Steps []Step
}

func defaultWorkflow() *Workflow {
	return &Workflow{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &Stage{
			Steps: []Step{
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
