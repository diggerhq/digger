package atlantis

// Represents an entire digger_config file
type AtlantisConfig struct {
	// Version of the digger_config syntax
	Version int `json:"version"`

	// If Atlantis should merge after finishing `atlantis apply`
	AutoMerge bool `json:"automerge"`

	// If Atlantis should allow plans to occur in parallel
	ParallelPlan bool `json:"parallel_plan"`

	// If Atlantis should allow applies to occur in parallel
	ParallelApply bool `json:"parallel_apply"`

	// The project settings
	Projects []AtlantisProject `json:"projects,omitempty"`

	// Workflows, which are not managed by this library other than
	// the fact that this library preserves any existing workflows
	Workflows interface{} `json:"workflows,omitempty"`
}

// Represents an Atlantis Project directory
type AtlantisProject struct {
	// The directory with the terragrunt.hcl file
	Dir string `json:"dir"`

	// Define workflow name
	Workflow string `json:"workflow,omitempty"`

	// Define workspace name
	Workspace string `json:"workspace,omitempty"`

	// Define project name
	Name string `json:"name,omitempty"`

	// Autoplan settings for which plans affect other plans
	Autoplan AutoplanConfig `json:"autoplan"`

	// The terraform version to use for this project
	TerraformVersion string `json:"terraform_version,omitempty"`

	// We only want to output `apply_requirements` if explicitly stated in a local value
	ApplyRequirements *[]string `json:"apply_requirements,omitempty"`

	// Atlantis use ExecutionOrderGroup for sort projects before applying/planning
	ExecutionOrderGroup int `json:"execution_order_group,omitempty"`
}

type AutoplanConfig struct {
	// Relative paths from this modules directory to modules it depends on
	WhenModified []string `json:"when_modified"`

	// If autoplan should be enabled for this dir
	Enabled bool `json:"enabled"`
}
