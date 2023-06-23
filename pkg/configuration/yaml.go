package configuration

import (
	"digger/pkg/core/models"
	"gopkg.in/yaml.v3"
)

type DiggerConfigYaml struct {
	Projects               []*ProjectYaml              `yaml:"projects"`
	AutoMerge              *bool                       `yaml:"auto_merge"`
	Workflows              map[string]*WorkflowYaml    `yaml:"workflows"`
	CollectUsageData       *bool                       `yaml:"collect_usage_data,omitempty"`
	GenerateProjectsConfig *GenerateProjectsConfigYaml `yaml:"generate_projects"`
}

type ProjectYaml struct {
	Name               string   `yaml:"name"`
	Dir                string   `yaml:"dir"`
	Workspace          string   `yaml:"workspace"`
	Terragrunt         bool     `yaml:"terragrunt"`
	Workflow           string   `yaml:"workflow"`
	IncludePatterns    []string `yaml:"include_patterns,omitempty"`
	ExcludePatterns    []string `yaml:"exclude_patterns,omitempty"`
	DependencyProjects []string `yaml:"dependency_projects,omitempty"`
}

type WorkflowYaml struct {
	EnvVars       *TerraformEnvConfigYaml    `yaml:"env_vars"`
	Plan          *StageYaml                 `yaml:"plan,omitempty"`
	Apply         *StageYaml                 `yaml:"apply,omitempty"`
	Configuration *WorkflowConfigurationYaml `yaml:"workflow_configuration"`
}

type WorkflowConfigurationYaml struct {
	OnPullRequestPushed []string `yaml:"on_pull_request_pushed"`
	OnPullRequestClosed []string `yaml:"on_pull_request_closed"`
	OnCommitToDefault   []string `yaml:"on_commit_to_default"`
}

func (s *StageYaml) ToCoreStage() models.Stage {
	var steps []models.Step
	for _, step := range s.Steps {
		steps = append(steps, step.ToCoreStep())
	}
	return models.Stage{Steps: steps}
}

type StageYaml struct {
	Steps []StepYaml `yaml:"steps"`
}

type StepYaml struct {
	Action    string
	Value     string
	ExtraArgs []string `yaml:"extra_args,omitempty"`
	Shell     string
}

type TerraformEnvConfigYaml struct {
	State    []EnvVarYaml `yaml:"state"`
	Commands []EnvVarYaml `yaml:"commands"`
}

type EnvVarYaml struct {
	Name      string `yaml:"name"`
	ValueFrom string `yaml:"value_from"`
	Value     string `yaml:"value"`
}

type GenerateProjectsConfigYaml struct {
	Include string `yaml:"include"`
	Exclude string `yaml:"exclude"`
}

func (p *ProjectYaml) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawProject ProjectYaml
	raw := rawProject{
		Workspace:  "default",
		Terragrunt: false,
		Workflow:   "default",
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*p = ProjectYaml(raw)
	return nil
}

func (w *WorkflowYaml) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawWorkflow WorkflowYaml
	raw := rawWorkflow{
		Configuration: &WorkflowConfigurationYaml{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &StageYaml{
			Steps: []StepYaml{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &StageYaml{
			Steps: []StepYaml{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
		EnvVars: &TerraformEnvConfigYaml{
			State:    []EnvVarYaml{},
			Commands: []EnvVarYaml{},
		},
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*w = WorkflowYaml(raw)
	return nil
}

func (s *StepYaml) UnmarshalYAML(value *yaml.Node) error {

	if value.Kind == yaml.ScalarNode {
		return value.Decode(&s.Action)
	}

	var stepMap map[string]interface{}
	if err := value.Decode(&stepMap); err != nil {
		return err
	}

	if _, ok := stepMap["run"]; ok {
		s.Action = "run"
		s.Value = stepMap["run"].(string)
		if _, ok := stepMap["shell"]; ok {
			s.Shell = stepMap["shell"].(string)
		}
		return nil
	}

	s.extract(stepMap, "plan")
	s.extract(stepMap, "apply")

	return nil
}

func (s *StepYaml) ToCoreStep() models.Step {
	return models.Step{
		Action:    s.Action,
		Value:     s.Value,
		ExtraArgs: s.ExtraArgs,
		Shell:     s.Shell,
	}
}

func (s *StepYaml) extract(stepMap map[string]interface{}, action string) {
	if _, ok := stepMap[action]; ok {
		s.Action = action
		var extraArgs []string
		if v, ok := stepMap["extra_args"]; ok {
			for _, v := range v.([]interface{}) {
				extraArgs = append(extraArgs, v.(string))
			}
			s.ExtraArgs = extraArgs
		} else {
			if v, ok := stepMap[action].(map[string]interface{})["extra_args"]; ok {
				for _, v := range v.([]interface{}) {
					extraArgs = append(extraArgs, v.(string))
				}
				s.ExtraArgs = extraArgs
			}
		}
	}
}
