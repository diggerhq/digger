package digger_config

import (
	"errors"

	"gopkg.in/yaml.v3"
)

type DiggerConfigYaml struct {
	ApplyAfterMerge            *bool                        `yaml:"apply_after_merge"`
	AllowDraftPRs              *bool                        `yaml:"allow_draft_prs"`
	DependencyConfiguration    *DependencyConfigurationYaml `yaml:"dependency_configuration"`
	PrLocks                    *bool                        `yaml:"pr_locks"`
	Projects                   []*ProjectYaml               `yaml:"projects"`
	AutoMerge                  *bool                        `yaml:"auto_merge"`
	CommentRenderMode          *string                      `yaml:"comment_render_mode"`
	Workflows                  map[string]*WorkflowYaml     `yaml:"workflows"`
	Telemetry                  *bool                        `yaml:"telemetry,omitempty"`
	GenerateProjectsConfig     *GenerateProjectsConfigYaml  `yaml:"generate_projects"`
	TraverseToNestedProjects   *bool                        `yaml:"traverse_to_nested_projects"`
	MentionDriftedProjectsInPR *bool                        `yaml:"mention_drifted_projects_in_pr"`
}

type DependencyConfigurationYaml struct {
	Mode string `yaml:"mode"`
}

type ProjectYaml struct {
	Name                 string                      `yaml:"name"`
	Dir                  string                      `yaml:"dir"`
	Workspace            string                      `yaml:"workspace"`
	Terragrunt           bool                        `yaml:"terragrunt"`
	OpenTofu             bool                        `yaml:"opentofu"`
	Pulumi               bool                        `yaml:"pulumi"`
	Workflow             string                      `yaml:"workflow"`
	WorkflowFile         string                      `yaml:"workflow_file"`
	IncludePatterns      []string                    `yaml:"include_patterns,omitempty"`
	ExcludePatterns      []string                    `yaml:"exclude_patterns,omitempty"`
	DependencyProjects   []string                    `yaml:"depends_on,omitempty"`
	DriftDetection       *bool                       `yaml:"drift_detection,omitempty"`
	AwsRoleToAssume      *AssumeRoleForProjectConfig `yaml:"aws_role_to_assume,omitempty"`
	Generated            bool                        `yaml:"generated"`
	AwsCognitoOidcConfig *AwsCognitoOidcConfig       `yaml:"aws_cognito_oidc,omitempty"`
	PulumiStack          string                      `yaml:"pulumi_stack"`
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
	// pull request converted to draft
	OnPullRequestConvertedToDraft []string `yaml:"on_pull_request_to_draft"`
	OnCommitToDefault             []string `yaml:"on_commit_to_default"`
	SkipMergeCheck                bool     `yaml:"skip_merge_check"`
}

func (s *StageYaml) ToCoreStage() Stage {
	var steps []Step
	for _, step := range s.Steps {
		steps = append(steps, step.ToCoreStep())
	}
	return Stage{Steps: steps}
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

type BlockYaml struct {
	// these flags are only for terraform and opentofu
	Include         string   `yaml:"include"`
	Exclude         string   `yaml:"exclude"`
	IncludePatterns []string `yaml:"include_patterns,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`

	// these flags are only for terragrunt
	Terragrunt bool    `yaml:"terragrunt"`
	RootDir    *string `yaml:"root_dir"`

	// these flags are only for opentofu
	OpenTofu bool `yaml:"opentofu"`

	// common flags
	Workspace            string                      `yaml:"workspace"`
	BlockName            string                      `yaml:"block_name"`
	Workflow             string                      `yaml:"workflow"`
	WorkflowFile         string                      `yaml:"workflow_file"`
	AwsRoleToAssume      *AssumeRoleForProjectConfig `yaml:"aws_role_to_assume,omitempty"`
	AwsCognitoOidcConfig *AwsCognitoOidcConfig       `yaml:"aws_cognito_oidc,omitempty"`
}

type AssumeRoleForProjectConfig struct {
	AwsRoleRegion string `yaml:"aws_role_region"`
	State         string `yaml:"state"`
	Command       string `yaml:"command"`
}

type AwsCognitoOidcConfig struct {
	AwsAccountId    string `yaml:"aws_account_id"`
	AwsRegion       string `yaml:"aws_region,omitempty"`
	CognitoPoolId   string `yaml:"cognito_identity_pool_id"`
	SessionDuration int    `yaml:"session_duration"`
}

type GenerateProjectsConfigYaml struct {
	Include                 string                      `yaml:"include"`
	Exclude                 string                      `yaml:"exclude"`
	Terragrunt              bool                        `yaml:"terragrunt"`
	Blocks                  []BlockYaml                 `yaml:"blocks"`
	TerragruntParsingConfig *TerragruntParsingConfig    `yaml:"terragrunt_parsing,omitempty"`
	AwsRoleToAssume         *AssumeRoleForProjectConfig `yaml:"aws_role_to_assume,omitempty"`
	AwsCognitoOidcConfig    *AwsCognitoOidcConfig       `yaml:"aws_cognito_oidc,omitempty"`
}

type TerragruntParsingConfig struct {
	GitRoot                    *string  `yaml:"gitRoot,omitempty"`
	AutoPlan                   bool     `yaml:"autoPlan"`
	AutoMerge                  bool     `yaml:"autoMerge"`
	IgnoreParentTerragrunt     *bool    `yaml:"ignoreParentTerragrunt,omitempty"`
	CreateParentProject        bool     `yaml:"createParentProject"`
	IgnoreDependencyBlocks     bool     `yaml:"ignoreDependencyBlocks"`
	TriggerProjectsFromDirOnly bool     `yaml:"triggerProjectsFromDirOnly"`
	Parallel                   *bool    `yaml:"parallel,omitempty"`
	CreateWorkspace            bool     `yaml:"createWorkspace"`
	CreateProjectName          bool     `yaml:"createProjectName"`
	DefaultTerraformVersion    string   `yaml:"defaultTerraformVersion"`
	DefaultWorkflow            string   `yaml:"defaultWorkflow"`
	FilterPath                 string   `yaml:"filterPath"`
	OutputPath                 string   `yaml:"outputPath"`
	PreserveWorkflows          *bool    `yaml:"preserveWorkflows,omitempty"`
	PreserveProjects           bool     `yaml:"preserveProjects"`
	CascadeDependencies        *bool    `yaml:"cascadeDependencies,omitempty"`
	DefaultApplyRequirements   []string `yaml:"defaultApplyRequirements"`
	//NumExecutors                   int64	`yaml:"numExecutors"`
	ProjectHclFiles                []string                    `yaml:"projectHclFiles"`
	CreateHclProjectChilds         bool                        `yaml:"createHclProjectChilds"`
	CreateHclProjectExternalChilds *bool                       `yaml:"createHclProjectExternalChilds,omitempty"`
	UseProjectMarkers              bool                        `yaml:"useProjectMarkers"`
	ExecutionOrderGroups           *bool                       `yaml:"executionOrderGroups"`
	WorkflowFile                   string                      `yaml:"workflow_file"`
	AwsRoleToAssume                *AssumeRoleForProjectConfig `yaml:"aws_role_to_assume,omitempty"`
	AwsCognitoOidcConfig           *AwsCognitoOidcConfig       `yaml:"aws_cognito_oidc,omitempty"`
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
			SkipMergeCheck:      false,
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
	if err := validateWorkflowConfigurationYaml(raw.Configuration); err != nil {
		return err
	}
	*w = WorkflowYaml(raw)
	return nil
}

func validateWorkflowConfigurationYaml(config *WorkflowConfigurationYaml) error {
	if config != nil {
		if config.OnPullRequestPushed == nil {
			return errors.New("workflow_configuration.on_pull_request_pushed is required")
		}
		if config.OnPullRequestClosed == nil {
			return errors.New("workflow_configuration.on_pull_request_closed is required")
		}
		if config.OnCommitToDefault == nil {
			return errors.New("workflow_configuration.on_commit_to_default is required")
		}
	}
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

	s.extract(stepMap, "init")
	s.extract(stepMap, "plan")
	s.extract(stepMap, "apply")

	return nil
}

func (s *StepYaml) ToCoreStep() Step {
	return Step{
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
			if stepMap[action] != nil {
				if v, ok := stepMap[action].(map[string]interface{})["extra_args"]; ok {
					for _, v := range v.([]interface{}) {
						extraArgs = append(extraArgs, v.(string))
					}
					s.ExtraArgs = extraArgs
				}
			}
		}
	}
}
