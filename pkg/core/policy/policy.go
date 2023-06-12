package policy

type Provider interface {
	GetPolicy(namespace string, projectname string) (string, error)
}

type Checker interface {
	Check(namespace string, projectname string, input interface{}) (bool, error)
}
