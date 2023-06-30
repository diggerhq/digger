package policy

import "digger/pkg/ci"

type Provider interface {
	GetPolicy(namespace string, projectname string) (string, error)
}

type Checker interface {
	Check(ciService ci.CIService, SCMOrganisation string, namespace string, projectname string, command string, requestedBy string) (bool, error)
}
