package reporting

import (
	"fmt"
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

// AsCollapsibleComment wraps a comment in a <details> block. The blank line
// after </summary> is required for markdown (links, bold, fences) to render
// inside an HTML block, and the content is deliberately not indented: four
// leading spaces would turn it into a markdown code block, and the
// strip-and-rewrap cycle in upsertComment accumulates indentation on every
// appended report.
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
