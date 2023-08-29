package policy

import orchestrator "github.com/diggerhq/lib-orchestrator"

type Provider interface {
	GetAccessPolicy(organisation string, repository string, projectname string) (string, error)
	GetPlanPolicy(organisation string, repository string, projectname string) (string, error)
	GetDriftPolicy() (string, error)
	GetOrganisation() string
}

type Checker interface {
	CheckAccessPolicy(ciService orchestrator.OrgService, SCMOrganisation string, SCMrepository string, projectname string, command string, requestedBy string) (bool, error)
	CheckPlanPolicy(SCMrepository string, projectname string, planOutput string) (bool, []string, error)
	CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error)
}
