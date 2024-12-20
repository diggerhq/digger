package reporting

type Reporter interface {
	Report(projectName string, report string, reportFormatting func(report string) string) (error error)
	Flush() ([]string, []string, error)
	Suppress(projectName string) error
	SupportsMarkdown() bool
}
