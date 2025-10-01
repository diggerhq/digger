package tfe

import "time"

// OrgAccessMatrix enumerates organization-scoped capability flags.
type OrgAccessMatrix struct {
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

type TFEOrganizationPermissions = OrgAccessMatrix

// OrgResource models an organization entity for a JSON:API client.
type OrgResource struct {
	// Resource identifier (JSON:API primary)
	Name string `jsonapi:"primary,organizations"`

	// Organization attributes
	AssessmentsEnforced                               bool                        `jsonapi:"attr,assessments-enforced" json:"assessments-enforced"`
	CollaboratorAuthPolicy                            string                      `jsonapi:"attr,collaborator-auth-policy" json:"collaborator-auth-policy"`
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

	// For older TFE builds (prior to 202211), this flag may be absent; deletions default to forced.
	AllowForceDeleteWorkspaces bool `jsonapi:"attr,allow-force-delete-workspaces" json:"allow-force-delete-workspaces"`
}

type TFEOrganization = OrgResource
