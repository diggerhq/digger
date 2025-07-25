package tac

// Represents an entire digger_config file
type AtlantisConfig struct {
	// Version of the digger_config syntax
	Version int `yaml:"version"`

	// If Atlantis should merge after finishing `atlantis apply`
	AutoMerge bool `yaml:"automerge"`

	// If Atlantis should allow plans to occur in parallel
	ParallelPlan bool `yaml:"parallel_plan"`

	// If Atlantis should allow applies to occur in parallel
	ParallelApply bool `yaml:"parallel_apply"`

	// The project settings
	Projects []AtlantisProject `yaml:"projects,omitempty"`

	// Workflows, which are not managed by this library other than
	// the fact that this library preserves any existing workflows
	Workflows interface{} `yaml:"workflows,omitempty"`
}

// Represents an Atlantis Project directory
type AtlantisProject struct {
	// The directory with the terragrunt.hcl file
	Dir string `yaml:"dir"`

	// Define workflow name
	Workflow string `yaml:"workflow,omitempty"`

	// Define workspace name
	Workspace string `yaml:"workspace,omitempty"`

	// Define project name
	Name string `yaml:"name,omitempty"`

	// Autoplan settings for which plans affect other plans
	Autoplan AutoplanConfig `yaml:"autoplan"`

	// The terraform version to use for this project
	TerraformVersion string `yaml:"terraform_version,omitempty"`

	// We only want to output `apply_requirements` if explicitly stated in a local value
	ApplyRequirements *[]string `yaml:"apply_requirements,omitempty"`

	// Atlantis use ExecutionOrderGroup for sort projects before applying/planning
	ExecutionOrderGroup int `yaml:"execution_order_group,omitempty"`
}

type AutoplanConfig struct {
	// Relative paths from this modules directory to modules it depends on
	WhenModified []string `yaml:"when_modified"`

	// If autoplan should be enabled for this dir
	Enabled bool `yaml:"enabled"`
}
