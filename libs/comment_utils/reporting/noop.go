package reporting

type NoopReporter struct{}

func (reporter NoopReporter) Report(projectName string, report string, reportFormatting func(report string) string) error {
	return nil
}

func (reporter NoopReporter) Flush() ([]string, []string, error) {
	return []string{}, []string{}, nil
}

func (reporter NoopReporter) SupportsMarkdown() bool {
	return false
}

func (reporter NoopReporter) Suppress(string) error {
	return nil
}
