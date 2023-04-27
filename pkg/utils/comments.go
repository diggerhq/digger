package utils

func GetTerraformOutputAsCollapsibleComment(summary string, collapsedComment string) string {
	str := `<details>
  <summary>` + summary + `</summary>

  ` + "```terraform" + `
` + collapsedComment + `
  ` + "```" + `
</details>`

	return str
}
