package digger_config

const CommentRenderModeBasic = "basic"
const CommentRenderModeGroupByModule = "group_by_module"

type AutomergeStrategy string

const AutomergeStrategySquash AutomergeStrategy = "squash"
const AutomergeStrategyMerge AutomergeStrategy = "merge"
const AutomergeStrategyRebase AutomergeStrategy = "rebase"

const DefaultBranchName = "__default__"

type DiggerConfig struct {
	ApplyAfterMerge               bool
	AllowDraftPRs                 bool
	CommentRenderMode             string
	DependencyConfiguration       DependencyConfiguration
	DeletePriorComments           bool
	DisableDiggerApplyComment     bool
	DisableDiggerApplyStatusCheck bool
	RespectLayers                 bool
	PrLocks                       bool
	Projects                      []Project
	AutoMerge                     bool
	AutoMergeStrategy             AutomergeStrategy
	Telemetry                     bool
	Workflows                     map[string]Workflow
	MentionDriftedProjectsInPR    bool
	TraverseToNestedProjects      bool
	Reporting                     ReporterConfig
	ReportTerraformOutputs        bool
}

type ReporterConfig struct {
	AiSummary bool
}

type DependencyConfiguration struct {
	Mode string
}

type AssumeRoleForProject struct {
	AwsRoleRegion string
	State         string
	Command       string
}

type Project struct {
	BlockName            string // the block name if this is a generated project
	Name                 string
	Branch               string
	Alias                string
	ApplyRequirements    []string
	Dir                  string
	Workspace            string
	Terragrunt           bool
	Layer                uint
	OpenTofu             bool
	Pulumi               bool
	Workflow             string
	WorkflowFile         string
	IncludePatterns      []string
	ExcludePatterns      []string
	DependencyProjects   []string
	DriftDetection       bool
	AwsRoleToAssume      *AssumeRoleForProject
	AwsCognitoOidcConfig *AwsCognitoOidcConfig
	Generated            bool
	PulumiStack          string
}

type Workflow struct {
	EnvVars       *TerraformEnvConfig
	Plan          *Stage
	Apply         *Stage
	Configuration *WorkflowConfiguration
}

type WorkflowConfiguration struct {
	OnPullRequestPushed           []string
	OnPullRequestClosed           []string
	OnPullRequestConvertedToDraft []string
	OnCommitToDefault             []string
	SkipMergeCheck                bool
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
	Steps       []Step
	FilterRegex *string
}

func defaultWorkflow() *Workflow {
	return &Workflow{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:             []string{"digger unlock"},
			OnPullRequestPushed:           []string{"digger plan"},
			OnPullRequestConvertedToDraft: []string{},
			OnPullRequestClosed:           []string{"digger unlock"},
			SkipMergeCheck:                false,
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
