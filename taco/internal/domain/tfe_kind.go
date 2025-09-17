package domain

type Kind string

func (k Kind) String() string {
	return string(k)
}

const (
	SiteKind                      Kind = "site"
	OrganizationKind              Kind = "org"
	WorkspaceKind                 Kind = "ws"
	RunKind                       Kind = "run"
	ConfigVersionKind             Kind = "cv"
	IngressAttributesKind         Kind = "ia"
	JobKind                       Kind = "job"
	ChunkKind                     Kind = "chunk"
	UserKind                      Kind = "user"
	TeamKind                      Kind = "team"
	ModuleKind                    Kind = "mod"
	ModuleVersionKind             Kind = "modver"
	NotificationConfigurationKind Kind = "nc"
	AgentPoolKind                 Kind = "apool"
	RunnerKind                    Kind = "runner"
	StateVersionKind              Kind = "sv"
	StateVersionOutputKind        Kind = "wsout"
	VariableSetKind               Kind = "varset"
	VariableKind                  Kind = "var"
	VCSProviderKind               Kind = "vcs"

	OrganizationTokenKind Kind = "ot"
	UserTokenKind         Kind = "ut"
	TeamTokenKind         Kind = "tt"
	AgentTokenKind        Kind = "at"
)
