package spec

type StepJson struct {
	Action    string   `json:"action"`
	Value     string   `json:"value"`
	ExtraArgs []string `json:"extraArgs"`
	Shell     string   `json:"shell"`
}

type StageJson struct {
	Steps []StepJson `json:"steps"`
}

type JobSpec struct {
	JobType           string            `json:"job_type"`
	ProjectName       string            `json:"projectName"`
	ProjectDir        string            `json:"projectDir"`
	ProjectWorkspace  string            `json:"projectWorkspace"`
	Terragrunt        bool              `json:"terragrunt"`
	OpenTofu          bool              `json:"opentofu"`
	Commands          []string          `json:"commands"`
	ApplyStage        StageJson         `json:"applyStage"`
	PlanStage         StageJson         `json:"planStage"`
	PullRequestNumber *int              `json:"pullRequestNumber"`
	EventName         string            `json:"eventName"`
	RequestedBy       string            `json:"requestedBy"`
	Namespace         string            `json:"namespace"`
	RunEnvVars        map[string]string `json:"runEnvVars"`
	StateEnvVars      map[string]string `json:"stateEnvVars"`
	CommandEnvVars    map[string]string `json:"commandEnvVars"`
	AwsRoleRegion     string            `json:"aws_role_region"`
	StateRoleName     string            `json:"state_role_name"`
	CommandRoleName   string            `json:"command_role_name"`
}

type ReporterSpec struct {
}

type LockSpec struct {
	lockType string `json:"lock_type"`
}

type BackendSpec struct {
	BackendHostname         string `json:"backend_hostname"`
	BackendOrganisationName string `json:"backend_organisation_hostname"`
	BackendJobToken         string `json:"backend_job_token"`
}

type Spec struct {
	Job      JobSpec      `json:"job"`
	reporter ReporterSpec `json:"reporter"`
	lock     LockSpec     `json:"lock"`
	backend  BackendSpec  `json:"backend"`
}
