package reporting

type Reporter interface {
	Report(report string) error
}
