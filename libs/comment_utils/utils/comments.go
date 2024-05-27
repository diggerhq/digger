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
	return func(comment string) string {
		return fmt.Sprintf(`<details><summary>` + summary + `</summary>
  ` + comment + `
</details>`)
	}
}

func AsComment(summary string) func(string) string {
	return func(comment string) string {
		return summary + "\n" + comment
	}
}
