package reporting

type NoopReporter struct{}

func (reporter NoopReporter) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	return "", "", nil
}

func (reporter NoopReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (reporter NoopReporter) SupportsMarkdown() bool {
	return false
}

func (reporter NoopReporter) Suppress() error {
	return nil
}
