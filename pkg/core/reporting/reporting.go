package reporting

type Reporter interface {
	Report(report string, reportFormatting func(report string) string) error
	SupportsCollapsibleComments() bool
}
