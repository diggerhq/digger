package domain

import "time"

type VCSRepo struct {
	owner string
	name  string
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type WorkspaceVersion struct {
	Latest bool   `json:"latest"`
	semver string `json:"semver"`
}

// ---- JSON:API DTOs below ----

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEWorkspaceActions struct {
	IsDestroyable bool `json:"is-destroyable"`
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEWorkspacePermissions struct {
	CanDestroy        bool `json:"can-destroy"`
	CanForceUnlock    bool `json:"can-force-unlock"`
	CanLock           bool `json:"can-lock"`
	CanQueueApply     bool `json:"can-queue-apply"`
	CanQueueDestroy   bool `json:"can-queue-destroy"`
	CanQueueRun       bool `json:"can-queue-run"`
	CanReadSettings   bool `json:"can-read-settings"`
	CanUnlock         bool `json:"can-unlock"`
	CanUpdate         bool `json:"can-update"`
	CanUpdateVariable bool `json:"can-update-variable"`
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
// TFEVCSRepo is carried as a single attribute object on the workspace.
type TFEVCSRepo struct {
	Branch            string  `json:"branch"`
	DisplayIdentifier string  `json:"display-identifier"`
	Identifier        VCSRepo `json:"identifier"`
	IngressSubmodules bool    `json:"ingress-submodules"`
	OAuthTokenID      string  `json:"oauth-token-id"`
	RepositoryHTTPURL string  `json:"repository-http-url"`
	TagsRegex         string  `json:"tags-regex"`
	ServiceProvider   string  `json:"service-provider"`
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFERun struct {
	ID string `jsonapi:"primary,runs"`
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEWorkspace struct {
	ID                         string                   `jsonapi:"primary,workspaces"`
	Actions                    *TFEWorkspaceActions     `jsonapi:"attr,actions" json:"actions"`
	AgentPoolID                string                   `jsonapi:"attr,agent-pool-id" json:"agent-pool-id"`
	AllowDestroyPlan           bool                     `jsonapi:"attr,allow-destroy-plan" json:"allow-destroy-plan"`
	AutoApply                  bool                     `jsonapi:"attr,auto-apply" json:"auto-apply"`
	CanQueueDestroyPlan        bool                     `jsonapi:"attr,can-queue-destroy-plan" json:"can-queue-destroy-plan"`
	CreatedAt                  time.Time                `jsonapi:"attr,created-at" json:"created-at"`
	Description                string                   `jsonapi:"attr,description" json:"description"`
	Environment                string                   `jsonapi:"attr,environment" json:"environment"`
	ExecutionMode              string                   `jsonapi:"attr,execution-mode" json:"execution-mode"`
	FileTriggersEnabled        bool                     `jsonapi:"attr,file-triggers-enabled" json:"file-triggers-enabled"`
	GlobalRemoteState          bool                     `jsonapi:"attr,global-remote-state" json:"global-remote-state"`
	Locked                     bool                     `jsonapi:"attr,locked" json:"locked"`
	MigrationEnvironment       string                   `jsonapi:"attr,migration-environment" json:"migration-environment"`
	Name                       string                   `jsonapi:"attr,Name" json:"Name"`
	Operations                 bool                     `jsonapi:"attr,operations" json:"operations"`
	Permissions                *TFEWorkspacePermissions `jsonapi:"attr,permissions" json:"permissions"`
	QueueAllRuns               bool                     `jsonapi:"attr,queue-all-runs" json:"queue-all-runs"`
	SpeculativeEnabled         bool                     `jsonapi:"attr,speculative-enabled" json:"speculative-enabled"`
	SourceName                 string                   `jsonapi:"attr,source-Name" json:"source-Name"`
	SourceURL                  string                   `jsonapi:"attr,source-url" json:"source-url"`
	StructuredRunOutputEnabled bool                     `jsonapi:"attr,structured-run-output-enabled" json:"structured-run-output-enabled"`
	TerraformVersion           *WorkspaceVersion        `jsonapi:"attr,terraform-version" json:"terraform-version"`
	TriggerPrefixes            []string                 `jsonapi:"attr,trigger-prefixes" json:"trigger-prefixes"`
	TriggerPatterns            []string                 `jsonapi:"attr,trigger-patterns" json:"trigger-patterns"`
	VCSRepo                    *TFEVCSRepo              `jsonapi:"attr,vcs-repo" json:"vcs-repo"`
	WorkingDirectory           string                   `jsonapi:"attr,working-directory" json:"working-directory"`
	UpdatedAt                  time.Time                `jsonapi:"attr,updated-at" json:"updated-at"`
	ResourceCount              int                      `jsonapi:"attr,resource-count" json:"resource-count"`
	ApplyDurationAverage       time.Duration            `jsonapi:"attr,apply-duration-average" json:"apply-duration-average"`
	PlanDurationAverage        time.Duration            `jsonapi:"attr,plan-duration-average" json:"plan-duration-average"`
	PolicyCheckFailures        int                      `jsonapi:"attr,policy-check-failures" json:"policy-check-failures"`
	RunFailures                int                      `jsonapi:"attr,run-failures" json:"run-failures"`
	RunsCount                  int                      `jsonapi:"attr,workspace-kpis-runs-count" json:"workspace-kpis-runs-count"`
	TagNames                   []string                 `jsonapi:"attr,tag-names" json:"tag-names"`

	// Relations
	CurrentRun   *TFERun               `jsonapi:"relation,current-run" json:"current-run"`
	Organization *TFEOrganization      `jsonapi:"relation,organization" json:"organization"`
	Outputs      []*TFEWorkspaceOutput `jsonapi:"relation,outputs" json:"outputs"`
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEWorkspaceOutput struct {
	ID        string `jsonapi:"primary,workspace-outputs"`
	Name      string `jsonapi:"attr,Name" json:"Name"`
	Sensitive bool   `jsonapi:"attr,sensitive" json:"sensitive"`
	Type      string `jsonapi:"attr,output-type" json:"output-type"`
	Value     any    `jsonapi:"attr,value" json:"value"`
}
