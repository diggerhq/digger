package policy

import "digger/pkg/ci"

type Provider interface {
	GetAccessPolicy(organisation string, repository string, projectname string) (string, error)
	GetPlanPolicy(organisation string, repository string, projectname string) (string, error)
	GetOrganisation() string
}

type Checker interface {
	CheckAccessPolicy(ciService ci.OrgService, SCMOrganisation string, SCMrepository string, projectname string, command string, requestedBy string) (bool, error)
	CheckPlanPolicy(SCMrepository string, projectname string, planOutput string) (bool, []string, error)
}
