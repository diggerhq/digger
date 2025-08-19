package utils

import "fmt"

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
	return `
<details>
  <summary>Instructions</summary>

▶️ To apply these changes, run the following command:

` + "```" + `bash
digger apply -p ` + projectName + `
` + "```" + `

⏩ To apply all changes in this PR:
` + "```" + `bash
digger apply
` + "```" + `

🚮 To unlock this project:
` + "```" + `bash
digger unlock -p ` + projectName + `
` + "```" + `

🚮 To unlock all projects in this PR:
` + "```" + `bash
digger unlock
` + "```" + `
</details>
`
}
