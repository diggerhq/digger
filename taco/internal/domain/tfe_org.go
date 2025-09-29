package domain

import "time"

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEOrganizationPermissions struct {
	CanCreateTeam               bool `json:"can-create-team"`
	CanCreateWorkspace          bool `json:"can-create-workspace"`
	CanCreateWorkspaceMigration bool `json:"can-create-workspace-migration"`
	CanDestroy                  bool `json:"can-destroy"`
	CanTraverse                 bool `json:"can-traverse"`
	CanUpdate                   bool `json:"can-update"`
	CanUpdateAPIToken           bool `json:"can-update-api-token"`
	CanUpdateOAuth              bool `json:"can-update-oauth"`
	CanUpdateSentinel           bool `json:"can-update-sentinel"`
}

type Name struct {
	Name string
}

func NewName(name string) Name {
	return Name{Name: name}
}

type TFEAuthPolicyType string

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEOrganization struct {
	// Primary ID
	Name string `jsonapi:"primary,organizations"`

	// Attributes
	AssessmentsEnforced                               bool                        `jsonapi:"attr,assessments-enforced" json:"assessments-enforced"`
	CollaboratorAuthPolicy                            TFEAuthPolicyType           `jsonapi:"attr,collaborator-auth-policy" json:"collaborator-auth-policy"`
	CostEstimationEnabled                             bool                        `jsonapi:"attr,cost-estimation-enabled" json:"cost-estimation-enabled"`
	CreatedAt                                         time.Time                   `jsonapi:"attr,created-at" json:"created-at"`
	Email                                             string                      `jsonapi:"attr,email" json:"email"`
	ExternalID                                        string                      `jsonapi:"attr,external-id" json:"external-id"`
	OwnersTeamSAMLRoleID                              string                      `jsonapi:"attr,owners-team-saml-role-id" json:"owners-team-saml-role-id"`
	Permissions                                       *TFEOrganizationPermissions `jsonapi:"attr,permissions" json:"permissions"`
	SAMLEnabled                                       bool                        `jsonapi:"attr,saml-enabled" json:"saml-enabled"`
	SessionRemember                                   *int                        `jsonapi:"attr,session-remember" json:"session-remember"`
	SessionTimeout                                    *int                        `jsonapi:"attr,session-timeout" json:"session-timeout"`
	TrialExpiresAt                                    time.Time                   `jsonapi:"attr,trial-expires-at" json:"trial-expires-at"`
	TwoFactorConformant                               bool                        `jsonapi:"attr,two-factor-conformant" json:"two-factor-conformant"`
	SendPassingStatusesForUntriggeredSpeculativePlans bool                        `jsonapi:"attr,send-passing-statuses-for-untriggered-speculative-plans" json:"send-passing-statuses-for-untriggered-speculative-plans"`
	RemainingTestableCount                            int                         `jsonapi:"attr,remaining-testable-count" json:"remaining-testable-count"`

	// Note: false on TFE < v202211 where setting doesn’t exist (all deletes are force).
	AllowForceDeleteWorkspaces bool `jsonapi:"attr,allow-force-delete-workspaces" json:"allow-force-delete-workspaces"`

	// Relations
	// DefaultProject *Project `jsonapi:"relation,default-project"`
}

// Adapted from OTF (MPL License): https://github.com/leg100/otf
type TFEEntitlements struct {
	ID                    string `jsonapi:"primary,entitlement-sets"`
	Agents                bool   `jsonapi:"attr,agents" json:"agents"`
	AuditLogging          bool   `jsonapi:"attr,audit-logging" json:"audit-logging"`
	CostEstimation        bool   `jsonapi:"attr,cost-estimation" json:"cost-estimation"`
	Operations            bool   `jsonapi:"attr,operations" json:"operations"`
	PrivateModuleRegistry bool   `jsonapi:"attr,private-module-registry" json:"private-module-registry"`
	SSO                   bool   `jsonapi:"attr,sso" json:"sso"`
	Sentinel              bool   `jsonapi:"attr,sentinel" json:"sentinel"`
	StateStorage          bool   `jsonapi:"attr,state-storage" json:"state-storage"`
	Teams                 bool   `jsonapi:"attr,teams" json:"teams"`
	VCSIntegrations       bool   `jsonapi:"attr,vcs-integrations" json:"vcs-integrations"`
}
