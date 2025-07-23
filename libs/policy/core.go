package policy

import (
	"github.com/diggerhq/digger/libs/ci"
)

type Provider interface {
	GetAccessPolicy(organisation, repository, projectname, projectDir string) (string, error)
	GetPlanPolicy(organisation, repository, projectname, projectDir string) (string, error)
	GetDriftPolicy() (string, error)
	GetOrganisation() string // TODO: remove this method from here since out of place
}

type Checker interface {
	// TODO refactor arguments - use AccessPolicyContext
	CheckAccessPolicy(ciService ci.OrgService, prService *ci.PullRequestService, SCMOrganisation, SCMrepository, projectName, projectDir, command string, prNumber *int, requestedBy string, planPolicyViolations []string) (bool, error)
	CheckPlanPolicy(SCMrepository, SCMOrganisation, projectname, projectDir, planOutput string) (bool, []string, error)
	CheckDriftPolicy(SCMOrganisation, SCMrepository, projectname string) (bool, error)
}

type PolicyCheckerProvider interface {
	Get(hostname, organisationName, authToken string) (Checker, error)
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
