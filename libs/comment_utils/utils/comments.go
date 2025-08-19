package utils

import (
	"fmt"
	"strings"
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

// FormatExampleCommands creates a collapsible markdown section with example commands
// for applying or unlocking a specific project
func FormatExampleCommands(projectName string) string {

	// Escape special shell characters to prevent command injection
	escapedProjectName := projectName
	// Escape backticks
	escapedProjectName = strings.Replace(escapedProjectName, "`", "\\`", -1)
	// Escape spaces, quotes, dollar signs, and other special shell characters
	escapedProjectName = strings.Replace(escapedProjectName, " ", "\\ ", -1)
	escapedProjectName = strings.Replace(escapedProjectName, "\"", "\\\"", -1)
	escapedProjectName = strings.Replace(escapedProjectName, "'", "\\'", -1)
	escapedProjectName = strings.Replace(escapedProjectName, "$", "\\$", -1)
	escapedProjectName = strings.Replace(escapedProjectName, "&", "\\&", -1)
	escapedProjectName = strings.Replace(escapedProjectName, "|", "\\|", -1)
	escapedProjectName = strings.Replace(escapedProjectName, ";", "\\;", -1)
	escapedProjectName = strings.Replace(escapedProjectName, "(", "\\(", -1)
	escapedProjectName = strings.Replace(escapedProjectName, ")", "\\)", -1)

	return `

▶️ To apply these changes, run the following command:

` + "```" + `bash
digger apply -p ` + escapedProjectName + `
` + "```" + `

⏩ To apply all changes in this PR:
` + "```" + `bash
digger apply
` + "```" + `

🚮 To unlock all projects in this PR:
` + "```" + `bash
digger unlock
` + "```" + `

`
}
