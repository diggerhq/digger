package utils

func GetTerraformOutputAsCollapsibleComment(summary string) func(string) string {

	return func(comment string) string {
		return `<details><summary>` + summary + `</summary>
  
` + "```terraform" + `
` + comment + `
  ` + "```" + `
</details>`
	}
}

func GetTerraformOutputAsComment(summary string) func(string) string {
	return func(comment string) string {
		return summary + "\n```terraform\n" + comment + "\n```"
	}
}

func AsCollapsibleComment(summary string) func(string) string {

	return func(comment string) string {
		return `<details><summary>` + summary + `</summary>
  ` + comment + `
</details>`
	}
}

func AsComment(summary string) func(string) string {
	return func(comment string) string {
		return summary + "\n" + comment
	}
}
