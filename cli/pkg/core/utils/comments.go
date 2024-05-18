package utils

import "fmt"

func GetTerraformOutputAsCollapsibleComment(summary string, open bool) func(string) string {

	return func(comment string) string {
		return fmt.Sprintf(`<details open="%v"><summary>`+summary+`</summary>

`+"```terraform"+`
`+comment+`
`+"```"+`
</details>`, open)
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
