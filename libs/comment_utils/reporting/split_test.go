package reporting

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// withTestSeparators swaps in short, predictable separators so expected
// outputs stay readable, restoring the real ones after the test.
func withTestSeparators(t *testing.T) {
	original := generateSeparatorsFunc
	generateSeparatorsFunc = func(fence string) map[closureType]separatorSet {
		return map[closureType]separatorSet{
			noClosure: {
				sepEnd:           "<!-- END -->",
				sepStart:         "<!-- START -->",
				truncationHeader: "<!-- TRUNCATED -->",
			},
			inCodeBlock: {
				sepEnd:           "```\n<!-- END -->",
				sepStart:         "```terraform\n<!-- START -->",
				truncationHeader: "```terraform\n<!-- TRUNCATED -->",
			},
			inDetailsBlock: {
				sepEnd:           "</details>\n<!-- END -->",
				sepStart:         "<details><summary>Show Output</summary>\n<!-- START -->",
				truncationHeader: "<details><summary>Show Output</summary>\n<!-- TRUNCATED -->",
			},
			inCodeInDetails: {
				sepEnd:           "```\n</details>\n<!-- END -->",
				sepStart:         "<details><summary>Show Output</summary>\n\n```terraform\n<!-- START -->",
				truncationHeader: "<details><summary>Show Output</summary>\n\n```terraform\n<!-- TRUNCATED -->",
			},
			inInlineCode: {
				sepEnd:           "`\n<!-- END -->",
				sepStart:         "<!-- START -->`",
				truncationHeader: "<!-- TRUNCATED -->`",
			},
		}
	}
	t.Cleanup(func() { generateSeparatorsFunc = original })
}

// reassemble strips test separators from chunks and joins them back together
func reassemble(chunks []string) string {
	startSeps := []string{
		"<details><summary>Show Output</summary>\n\n```terraform\n<!-- START -->",
		"<details><summary>Show Output</summary>\n<!-- START -->",
		"```terraform\n<!-- START -->",
		"<!-- START -->`",
		"<!-- START -->",
	}
	endSeps := []string{
		"```\n</details>\n<!-- END -->",
		"</details>\n<!-- END -->",
		"```\n<!-- END -->",
		"`\n<!-- END -->",
		"<!-- END -->",
	}
	var sb strings.Builder
	for _, chunk := range chunks {
		for _, sep := range startSeps {
			if strings.HasPrefix(chunk, sep) {
				chunk = strings.TrimPrefix(chunk, sep)
				break
			}
		}
		for _, sep := range endSeps {
			if strings.HasSuffix(chunk, sep) {
				chunk = strings.TrimSuffix(chunk, sep)
				break
			}
		}
		sb.WriteString(chunk)
	}
	return sb.String()
}

func TestSplitComment(t *testing.T) {
	withTestSeparators(t)

	tests := []struct {
		name        string
		comment     string
		maxSize     int
		maxComments int
	}{
		{
			name:        "TwoComments",
			comment:     strings.Repeat("a", 1000),
			maxSize:     999,
			maxComments: 0,
		},
		{
			name:        "MultipleComments",
			comment:     strings.Repeat("a", 1000),
			maxSize:     300,
			maxComments: 0,
		},
		{
			name:        "NoClosureText",
			comment:     "This is a long comment that will be split. " + strings.Repeat("This is additional content to make the comment longer so it will be split. ", 5),
			maxSize:     70,
			maxComments: 0,
		},
		{
			name:        "CodeBlock",
			comment:     "Here's some code:\n```\nterraform plan\noutput here\n```\nAnd more text. " + strings.Repeat("This is additional content to make the comment longer so it will be split. ", 3),
			maxSize:     200,
			maxComments: 0,
		},
		{
			name:        "DetailsBlock",
			comment:     "<details><summary>Show Output</summary>\n\nSome details content here. " + strings.Repeat("This is additional content to make the comment longer so it will be split. ", 4) + "\n</details>",
			maxSize:     200,
			maxComments: 0,
		},
		{
			name: "CodeInDetails",
			comment: "<details><summary>Show Output</summary>\n\n```terraform\n" +
				strings.Repeat("Line of terraform output\n", 20) +
				"```\n</details>",
			maxSize:     220,
			maxComments: 0,
		},
		{
			name:        "InlineCode",
			comment:     "Here is text: `" + strings.Repeat("a", 150) + "` end.",
			maxSize:     100,
			maxComments: 0,
		},
		{
			name:        "DetailsBlockWithAttributes",
			comment:     "<details open><summary>Show Output</summary>\n\nSome details content here. " + strings.Repeat("This is additional content to make the comment longer so it will be split. ", 4) + "\n</details>",
			maxSize:     200,
			maxComments: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			split := SplitComment(tt.comment, tt.maxSize, tt.maxComments)

			assert.Greater(t, len(split), 1, "comment above maxSize must be split")
			for i, chunk := range split {
				assert.LessOrEqualf(t, len(chunk), tt.maxSize, "chunk %d exceeds maxSize", i)
			}
			// stripping separators must reassemble the original comment
			assert.Equal(t, tt.comment, reassemble(split))
			// every chunk except the last announces a continuation, every
			// chunk except the first is marked as one
			for _, chunk := range split[:len(split)-1] {
				assert.Contains(t, chunk, "<!-- END -->")
			}
			for _, chunk := range split[1:] {
				assert.Contains(t, chunk, "<!-- START -->")
			}
		})
	}
}

func TestSplitCommentUnderMaxSizeUnchanged(t *testing.T) {
	withTestSeparators(t)
	comment := "comment under max size"
	assert.Equal(t, []string{comment}, SplitComment(comment, 50, 0))
}

func TestSplitCommentExactOutput(t *testing.T) {
	withTestSeparators(t)

	// backwards construction: the last chunk is filled first
	// (sepStart is 14 chars: 14 + 985 = 999), the remainder lands in the
	// first chunk with sepEnd appended (15 + 12 = 27)
	split := SplitComment(strings.Repeat("a", 1000), 999, 0)

	assert.Equal(t, []string{
		strings.Repeat("a", 15) + "<!-- END -->",
		"<!-- START -->" + strings.Repeat("a", 985),
	}, split)
}

func TestSplitCommentTruncationPreservesTail(t *testing.T) {
	withTestSeparators(t)

	tail := "Plan: 3 to add, 1 to change, 0 to destroy."
	comment := strings.Repeat("head ", 100) + strings.Repeat("body ", 100) + tail
	split := SplitComment(comment, 120, 2)

	assert.Len(t, split, 2)
	assert.Contains(t, split[0], "<!-- TRUNCATED -->")
	assert.True(t, strings.HasSuffix(split[len(split)-1], tail), "tail of the comment must survive truncation")
	for i, chunk := range split {
		assert.LessOrEqualf(t, len(chunk), 120, "chunk %d exceeds maxSize", i)
	}
}

func TestSplitCommentRealSeparatorsReopenTerraformFence(t *testing.T) {
	// real separators: a plan inside <details>...```terraform must reopen the
	// fence in the continuation chunk
	comment := "<details open><summary>Plan output</summary>\n\n```terraform\n" +
		strings.Repeat("  + resource \"aws_instance\" \"example\"\n", 100) +
		"```\n</details>"
	maxSize := 1000
	split := SplitComment(comment, maxSize, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split {
		assert.LessOrEqualf(t, len(chunk), maxSize, "chunk %d exceeds maxSize", i)
		// every chunk must close what it opens: even number of ``` fences
		assert.Equalf(t, 0, strings.Count(chunk, "```")%2, "chunk %d has unbalanced code fences", i)
	}
	for _, chunk := range split[1:] {
		assert.True(t, strings.HasPrefix(chunk, "Continued from [previous comment]("+prevCommentUrlPlaceholder+")."))
		assert.Contains(t, chunk, "```terraform")
	}
	for _, chunk := range split[:len(split)-1] {
		assert.Contains(t, chunk, "Continued in next comment.")
	}
}

func TestSplitCommentDoesNotSplitMidRune(t *testing.T) {
	// no whitespace at all, multi-byte runes throughout: forces arbitrary
	// position splits which must back off to rune boundaries
	comment := strings.Repeat("héllo→wörld🌍", 500)
	maxSize := 300
	split := SplitComment(comment, maxSize, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split {
		assert.LessOrEqualf(t, len(chunk), maxSize, "chunk %d exceeds maxSize", i)
		assert.Truef(t, utf8.ValidString(chunk), "chunk %d contains invalid UTF-8", i)
	}
}

func TestSplitCommentSeparatorsTooLargeForMaxSize(t *testing.T) {
	comment := strings.Repeat("a", 1000)
	// smaller than any separator pair: best effort, return unsplit
	split := SplitComment(comment, 10, 0)
	assert.Equal(t, []string{comment}, split)
}

func TestGenerateSeparatorsCoversAllClosureTypes(t *testing.T) {
	seps := generateSeparators("```")
	for _, ct := range []closureType{noClosure, inCodeBlock, inDetailsBlock, inCodeInDetails, inInlineCode} {
		set, ok := seps[ct]
		assert.Truef(t, ok, "missing separators for closure type %d", ct)
		assert.NotEmpty(t, set.sepEnd)
		assert.NotEmpty(t, set.sepStart)
		assert.NotEmpty(t, set.truncationHeader)
	}
}

func TestDetectClosureType(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		expected closureType
		fence    string
	}{
		{"plain text", "just text", noClosure, "```"},
		{"open code fence", "text\n```\ncode", inCodeBlock, "```"},
		{"closed code fence", "text\n```\ncode\n```\n", noClosure, "```"},
		{"open details", "<details><summary>x</summary>\nbody", inDetailsBlock, "```"},
		{"closed details", "<details><summary>x</summary>\nbody</details>", noClosure, "```"},
		{"code in details", "<details><summary>x</summary>\n```\ncode", inCodeInDetails, "```"},
		{"details with attrs", "<details open><summary>x</summary>\nbody", inDetailsBlock, "```"},
		{"inline code", "some `inline", inInlineCode, "```"},
		{"details token inside code fence ignored", "```\n<details>\n", inCodeBlock, "```"},
		// fences only count at line start: a mid-line ``` is not a fence
		{"mid-line fence marker is not a fence", "a `x` b `y` ```\ncode", noClosure, "```"},
		{"mid-line marker inside open fence stays open", "```terraform\n  x = \"use ``` here\"\ncode", inCodeBlock, "```"},
		{"indented fence up to 3 spaces counts", "   ```\ncode", inCodeBlock, "```"},
		{"deeply indented fence is content", "    ```\ntext", noClosure, "```"},
		// a closing fence must be at least as long as the opening one
		{"longer fence needs longer close", "````\n```\ncode", inCodeBlock, "````"},
		{"longer fence closed by equal run", "````\n```\n````\ntext", noClosure, "```"},
		// tilde fences are fences too, and mix with backtick content
		{"tilde fence", "~~~\ncode", inCodeBlock, "~~~"},
		{"backtick fence inside tilde fence is content", "~~~\n```\ncode", inCodeBlock, "~~~"},
		// an unmatched backtick stays local to its line
		{"stray backtick does not leak to later lines", "a ` stray\nplain", noClosure, "```"},
		{"backtick fence with info string containing backtick is not a fence", "``` foo ` bar\ntext", noClosure, "```"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, fence := detectClosureType(tt.comment, len(tt.comment))
			assert.Equal(t, tt.expected, ct)
			assert.Equal(t, tt.fence, fence)
		})
	}
}

// rendererFenceBalanced mimics how the VCS actually parses fences: a fence
// token only opens or closes a code block when it sits at the start of a
// line (up to 3 leading spaces).
func rendererFenceBalanced(chunk string) bool {
	open := false
	for _, line := range strings.Split(chunk, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) <= 3 && strings.HasPrefix(trimmed, "```") {
			open = !open
		}
	}
	return !open
}

func TestSplitCommentMidLineBackticksKeepFenceBalanced(t *testing.T) {
	// regression for the fence desync: a plan whose string value contains a
	// literal ``` mid-line must not desync the splitter from the renderer
	var body strings.Builder
	body.WriteString("  + description = \"use ``` to fence code blocks\"\n")
	for range 60 {
		body.WriteString("  + resource line\n")
	}
	comment := "<details open><summary>Plan output</summary>\n\n```terraform\n" +
		body.String() +
		"```\n</details>"

	split := SplitComment(comment, 700, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split {
		assert.Truef(t, rendererFenceBalanced(chunk), "chunk %d has unbalanced fences under line-anchored rendering", i)
	}
}

func TestSplitCommentReopensLongerFenceMarkers(t *testing.T) {
	// a ```` fence (which may legitimately contain ``` as content) must be
	// closed and reopened with the same four-backtick marker
	var body strings.Builder
	for range 60 {
		body.WriteString("content line\n```\nstill the same block\n")
	}
	comment := "````\n" + body.String() + "````"

	split := SplitComment(comment, 700, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split[:len(split)-1] {
		assert.Containsf(t, chunk, "\n````\n", "chunk %d must close with the four-backtick marker", i)
	}
	for i, chunk := range split[1:] {
		assert.Containsf(t, chunk, "````terraform\n", "chunk %d must reopen with the four-backtick marker", i+1)
	}
}

func TestSplitCommentReopensTildeFence(t *testing.T) {
	var body strings.Builder
	for range 60 {
		body.WriteString("tilde fenced content line\n")
	}
	comment := "~~~\n" + body.String() + "~~~"

	split := SplitComment(comment, 700, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split[:len(split)-1] {
		assert.Containsf(t, chunk, "\n~~~\n", "chunk %d must close the tilde fence", i)
	}
	for i, chunk := range split[1:] {
		assert.Containsf(t, chunk, "~~~terraform\n", "chunk %d must reopen the tilde fence", i+1)
	}
}

func TestSplitCommentDetailsWithoutFenceReopensPlain(t *testing.T) {
	// splitting inside a <details> block that holds plain text must reopen
	// only the details block, not inject a code fence that nothing closes
	var body strings.Builder
	for range 60 {
		body.WriteString("plain text line inside details\n")
	}
	comment := "<details><summary>Notes</summary>\n\n" + body.String() + "</details>"

	split := SplitComment(comment, 700, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split {
		assert.NotContainsf(t, chunk, "```", "chunk %d must not contain a code fence", i)
	}
	for i, chunk := range split[1:] {
		assert.Containsf(t, chunk, "<details><summary>Continued output</summary>", "chunk %d must reopen the details block", i+1)
	}
}

func TestSplitCommentStrayBacktickDoesNotLeak(t *testing.T) {
	// an unmatched backtick early in plain text must not make later chunks
	// open with inline-code separators
	var body strings.Builder
	body.WriteString("there is a stray ` backtick here\n")
	for range 60 {
		body.WriteString("plain text line\n")
	}

	split := SplitComment(body.String(), 700, 0)

	assert.Greater(t, len(split), 1)
	for i, chunk := range split[1:] {
		assert.Truef(t, strings.HasPrefix(chunk, "Continued from [previous comment]("+prevCommentUrlPlaceholder+").\n"),
			"chunk %d must use the plain continuation header", i+1)
		afterHeader := strings.TrimPrefix(chunk, "Continued from [previous comment]("+prevCommentUrlPlaceholder+").\n")
		assert.Falsef(t, strings.HasPrefix(afterHeader, "`"), "chunk %d must not reopen inline code", i+1)
	}
}
