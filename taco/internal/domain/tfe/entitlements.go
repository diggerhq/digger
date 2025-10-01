package tfe

type FeatureEntitlements struct {
	ID string `jsonapi:"primary,entitlement-sets"`

	// Access and Security
	SSO          bool `jsonapi:"attr,sso" json:"sso"`
	Teams        bool `jsonapi:"attr,teams" json:"teams"`
	AuditLogging bool `jsonapi:"attr,audit-logging" json:"audit_logging"`

	// Infrastructure & Operations
	Agents       bool `jsonapi:"attr,agents" json:"agents"`
	Operations   bool `jsonapi:"attr,operations" json:"operations"`
	StateStorage bool `jsonapi:"attr,state-storage" json:"state_storage"`

	// Policy & Governance
	Sentinel bool `jsonapi:"attr,sentinel" json:"sentinel"`

	// Modules, Integrations & Cost
	PrivateModuleRegistry bool `jsonapi:"attr,private-module-registry" json:"private_module_registry"`
	VCSIntegrations       bool `jsonapi:"attr,vcs-integrations" json:"vcs_integrations"`
	CostEstimation        bool `jsonapi:"attr,cost-estimation" json:"cost_estimation"`
}

func DefaultFeatureEntitlements(orgId string) *FeatureEntitlements {
	return &FeatureEntitlements{
		ID:                    orgId,
		SSO:                   true,
		Teams:                 true,
		AuditLogging:          true,
		Agents:                true,
		Operations:            true,
		StateStorage:          true,
		Sentinel:              true,
		PrivateModuleRegistry: true,
		VCSIntegrations:       true,
		CostEstimation:        true,
	}
}
