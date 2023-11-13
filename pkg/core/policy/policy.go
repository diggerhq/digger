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
	CheckAccessPolicy(ciService orchestrator.OrgService, prService *orchestrator.PullRequestService, SCMOrganisation string, SCMrepository string, projectname string, command string, prNumber *int, requestedBy string) (bool, error)
	CheckPlanPolicy(SCMrepository string, projectname string, planOutput string) (bool, []string, error)
	CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error)
}
