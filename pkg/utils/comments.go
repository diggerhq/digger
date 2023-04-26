package utils

func GetCollapsibleComment(summary string, collapedComment string) string {
	str := `<details>
  <summary>` + summary + `</summary>
  
  ` + "```" + `
` + collapedComment + `
  ` + "```" + `
</details>`

	return str
}
