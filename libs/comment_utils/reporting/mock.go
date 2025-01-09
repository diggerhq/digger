package reporting

type MockReporter struct {
	commands []string
}

func (mockReporter *MockReporter) Report(projectName string, report string, reportFormatting func(report string) string) error {
	mockReporter.commands = append(mockReporter.commands, "Report")
	return nil
}

func (mockReporter *MockReporter) Flush() ([]string, []string, error) {
	return []string{}, []string{}, nil
}

func (mockReporter *MockReporter) Suppress(string) error {
	return nil
}

func (mockReporter *MockReporter) SupportsMarkdown() bool {
	mockReporter.commands = append(mockReporter.commands, "SupportsMarkdown")
	return false
}
