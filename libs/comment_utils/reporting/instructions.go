package reporting

import (
	"fmt"
	"strings"
)

func FormatAndReportExampleCommands(projectName string, reporter Reporter) error {
	// Escape special shell characters to prevent command injection
	escapedProjectName := strings.NewReplacer(
		"`", "\\`",
		" ", "\\ ",
		"\"", "\\\"",
		"'", "\\'",
		"$", "\\$",
		"&", "\\&",
		"|", "\\|",
		";", "\\;",
		"(", "\\(",
		")", "\\)",
	).Replace(projectName)

	commands := fmt.Sprintf(`
▶️ To apply these changes, run the following command:

`+"```"+`bash
digger apply -p %s
`+"```"+`

⏩ To apply all changes in this PR:
`+"```"+`bash
digger apply
`+"```"+`

🚮 To unlock all projects in this PR:
`+"```"+`bash
digger unlock
`+"```"+`
`, escapedProjectName)

	var formatter func(string) string
	if reporter.SupportsMarkdown() {
		formatter = AsCollapsibleComment("Instructions", false)
	} else {
		formatter = AsComment("Instructions")
	}

	_, _, err := reporter.Report(commands, formatter)
	return err
}
