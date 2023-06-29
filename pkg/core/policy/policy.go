package policy

type Provider interface {
	GetPolicy(namespace string, projectname string) (string, error)
}

type Checker interface {
	Check(githubOrganisation string, namespace string, projectname string, command string, requestedBy string) (bool, error)
}
