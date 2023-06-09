package policy

type Checker interface {
	Check(input interface{}) (bool, []string, error)
}
