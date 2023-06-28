package utils

func GetTerraformOutputAsCollapsibleComment(summary string) func(string) string {

	return func(comment string) string {
		return `<details>
  <summary>` + summary + `</summary>

  ` + "```terraform" + `
` + comment + `
  ` + "```" + `
</details>`
	}
}

func AsCollapsibleComment(summary string) func(string) string {

	return func(comment string) string {
		return `<details>
  <summary>` + summary + `</summary>
  ` + comment + `
</details>`
	}
}
