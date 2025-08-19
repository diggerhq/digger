package utils

import (
	"fmt"
	"strings"

	"github.com/diggerhq/digger/libs/comment_utils/reporting"
)

func GetTerraformOutputAsCollapsibleComment(summary string, open bool) func(string) string {
	var openTag string
	if open {
		openTag = "open=\"true\""
	} else {
		openTag = ""
	}

	return func(comment string) string {
		return fmt.Sprintf(`<details %v><summary>`+summary+`</summary>

`+"```terraform"+`
`+comment+`
`+"```"+`
</details>`, openTag)
	}
}

func GetTerraformOutputAsComment(summary string) func(string) string {
	return func(comment string) string {
		return summary + "\n```terraform\n" + comment + "\n```"
	}
}

func AsCollapsibleComment(summary string, open bool) func(string) string {
	var openTag string
	if open {
		openTag = "open=\"true\""
	} else {
		openTag = ""
	}
	return func(comment string) string {
		return fmt.Sprintf(`<details %v><summary>`+summary+`</summary>
  `+comment+`
</details>`, openTag)
	}
}

func AsComment(summary string) func(string) string {
	return func(comment string) string {
		return summary + "\n" + comment
	}
}

// FormatAndReportExampleCommands formats and reports the example commands using the provided reporter
func FormatAndReportExampleCommands(projectName string, reporter reporting.Reporter) error {
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
