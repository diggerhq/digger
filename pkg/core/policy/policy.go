package policy

import "digger/pkg/ci"

type Provider interface {
	GetPolicy(namespace string, projectname string) (string, error)
}

type Checker interface {
	Check(ciService ci.OrgService, SCMOrganisation string, SCMrepository string, projectname string, command string, requestedBy string) (bool, error)
}
