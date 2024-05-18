package reporting

type Reporter interface {
	Report(report string, reportFormatting func(report string) string) (commentId string, commentUrl string, error error)
	Flush() (string, string, error)
	Suppress() error
	SupportsMarkdown() bool
}
