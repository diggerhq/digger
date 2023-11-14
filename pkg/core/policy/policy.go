package policy

import (
	"github.com/diggerhq/digger/libs/orchestrator"
)

type Provider interface {
	GetAccessPolicy(organisation string, repository string, projectname string) (string, error)
	GetPlanPolicy(organisation string, repository string, projectname string) (string, error)
	GetDriftPolicy() (string, error)
	GetOrganisation() string // TODO: remove this method from here since out of place
}

type Checker interface {
	// TODO refactor arguments - use AccessPolicyContext
	CheckAccessPolicy(ciService orchestrator.OrgService, prService *orchestrator.PullRequestService, SCMOrganisation string, SCMrepository string, projectName string, command string, prNumber *int, requestedBy string, planPolicyViolations []string) (bool, error)
	CheckPlanPolicy(SCMrepository string, projectname string, planOutput string) (bool, []string, error)
	CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error)
}

type AccessPolicyContext struct {
	SCMOrganisation  string
	SCMrepository    string
	projectName      string
	command          string
	prNumber         *int
	requestedBy      string
	policyViolations []string
}
