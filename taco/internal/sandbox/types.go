package sandbox

import "context"

// PlanRequest bundles the inputs needed to execute a Terraform/OpenTofu plan inside a sandbox.
type PlanRequest struct {
	RunID                  string
	PlanID                 string
	OrgID                  string
	UnitID                 string
	ConfigurationVersionID string
	IsDestroy              bool
	TerraformVersion       string
	Engine                 string // "terraform" or "tofu"
	WorkingDirectory       string
	ConfigArchive          []byte
	State                  []byte
	Metadata               map[string]string
	// LogSink is an optional callback that receives incremental log chunks
	// as they are observed while polling the sandbox run.
	LogSink func(chunk string)
}

// PlanResult captures the outcome of a sandboxed plan execution.
type PlanResult struct {
	Logs                 string
	HasChanges           bool
	ResourceAdditions    int
	ResourceChanges      int
	ResourceDestructions int
	PlanJSON             []byte
	RuntimeRunID         string
}

// ApplyRequest bundles the inputs needed to execute a Terraform/OpenTofu apply inside a sandbox.
type ApplyRequest struct {
	RunID                  string
	PlanID                 string
	OrgID                  string
	UnitID                 string
	ConfigurationVersionID string
	IsDestroy              bool
	TerraformVersion       string
	Engine                 string // "terraform" or "tofu"
	WorkingDirectory       string
	ConfigArchive          []byte
	State                  []byte
	Metadata               map[string]string
	// LogSink is an optional callback that receives incremental log chunks
	// as they are observed while polling the sandbox run.
	LogSink func(chunk string)
}

// ApplyResult captures the outcome of a sandboxed apply.
type ApplyResult struct {
	Logs         string
	State        []byte
	RuntimeRunID string
}

// Sandbox defines the behavior any sandbox provider must implement.
type Sandbox interface {
	Name() string
	ExecutePlan(ctx context.Context, req *PlanRequest) (*PlanResult, error)
	ExecuteApply(ctx context.Context, req *ApplyRequest) (*ApplyResult, error)
}
