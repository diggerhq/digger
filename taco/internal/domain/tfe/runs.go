package tfe

type TFERun struct {
	ID string `jsonapi:"primary,runs" json:"id"`

	// ----- attributes -----
	Status        string          `jsonapi:"attr,status" json:"status"`
	HasChanges    bool            `jsonapi:"attr,has-changes" json:"has-changes"`
	IsDestroy     bool            `jsonapi:"attr,is-destroy" json:"is-destroy"`
	Message       string          `jsonapi:"attr,message" json:"message"`
	PlanOnly      bool            `jsonapi:"attr,plan-only" json:"plan-only"`
	AutoApply     bool            `jsonapi:"attr,auto-apply" json:"auto-apply"`
	IsConfirmable bool            `jsonapi:"attr,is-confirmable" json:"is-confirmable"`
	Actions       *RunActions     `jsonapi:"attr,actions" json:"actions"`
	Permissions   *RunPermissions `jsonapi:"attr,permissions" json:"permissions"`

	// ----- relationships -----
	Plan                 *PlanRef                 `jsonapi:"relation,plan" json:"plan"`
	Apply                *ApplyRef                `jsonapi:"relation,apply,omitempty" json:"apply,omitempty"`
	Workspace            *WorkspaceRef            `jsonapi:"relation,workspace" json:"workspace"`
	ConfigurationVersion *ConfigurationVersionRef `jsonapi:"relation,configuration-version" json:"configuration-version"`
}

// Actions block Terraform likes to see on runs
type RunActions struct {
	IsCancelable  bool `json:"is-cancelable"`
	IsConfirmable bool `json:"is-confirmable"`
}

type RunPermissions struct {
	CanApply bool `json:"can-apply"`
}

// Relationship: plan
type PlanRef struct {
	ID string `jsonapi:"primary,plans" json:"id"`
}

// Relationship: apply
type ApplyRef struct {
	ID string `jsonapi:"primary,applies" json:"id"`
}

// Relationship: workspace
type WorkspaceRef struct {
	ID string `jsonapi:"primary,workspaces" json:"id"`
}

// Relationship: configuration-version
type ConfigurationVersionRef struct {
	ID string `jsonapi:"primary,configuration-versions" json:"id"`
}
