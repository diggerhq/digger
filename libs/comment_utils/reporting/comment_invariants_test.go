package reporting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const terraformFenceInfo = "terraform"

// A rune sequence no plan output can contain.
const skeletonBlockSentinel = "\x00terraform-block\x00"

// The summary the reference fixture was captured with, sanitised.
const fixturePresignedPlanURL = `https://example-plans-bucket.s3.eu-central-1.amazonaws.com/example-sharedservices-account-134-shared-services.tfplan.txt?response-content-disposition=inline&response-content-type=text%2Fplain&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAIOSFODNN7EXAMPLE%2F20260903%2Feu-central-1%2Fs3%2Faws4_request&X-Amz-Date=20260903T120000Z&X-Amz-Expires=3600&X-Amz-Security-Token=IQoJb3JpZ2luX2VjEExampleTemporarySessionToken&X-Amz-SignedHeaders=host&X-Amz-Signature=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`
const fixtureEscapedPresignedPlanURL = `https://example-plans-bucket.s3.eu-central-1.amazonaws.com/example-sharedservices-account-134-shared-services.tfplan.txt?response-content-disposition=inline&amp;response-content-type=text%2Fplain&amp;X-Amz-Algorithm=AWS4-HMAC-SHA256&amp;X-Amz-Credential=ASIAIOSFODNN7EXAMPLE%2F20260903%2Feu-central-1%2Fs3%2Faws4_request&amp;X-Amz-Date=20260903T120000Z&amp;X-Amz-Expires=3600&amp;X-Amz-Security-Token=IQoJb3JpZ2luX2VjEExampleTemporarySessionToken&amp;X-Amz-SignedHeaders=host&amp;X-Amz-Signature=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`
const fixturePlanOutputSummary = `Plan output (<a href="` + fixtureEscapedPresignedPlanURL + `">full plan — valid for up to 1 hour</a>)`
const fixturePlainPlanOutputSummary = `Plan output - full plan: ` + fixturePresignedPlanURL

func readFixture(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

func snippet(s string) string {
	const max = 120
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

func fenceInfoString(line string) (info string, backticks int, ok bool) {
	body := strings.TrimLeft(line, " \t")
	if indent := line[:len(line)-len(body)]; len(indent) > 3 || strings.Contains(indent, "\t") {
		return "", 0, false // an indented code block, not a fence
	}
	trimmed := strings.TrimSpace(body)
	backticks = len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
	if backticks < 3 {
		return "", 0, false
	}
	return strings.TrimSpace(trimmed[backticks:]), backticks, true
}

// Splits body into the content of its ```terraform fences and everything else, which counts
// as protected prose — including the fence lines themselves and the content of non-terraform
// fences. Returns len(protected) == len(tfContents)+1.
//
// Every fence is tracked, not only terraform ones: a ```terraform line sitting inside a
// ```bash block is bash content, and reading it as an opening fence desynchronises the parse
// so every later block is misattributed. Fence length is tracked for the same reason: a bare ```
// inside a ````-opened block is content, not its closing delimiter.
func splitTerraformBlocks(body string) (protected []string, tfContents []string) {
	var current, tfContent []string
	inFence := false
	openInfo := ""
	openBackticks := 0

	for _, line := range strings.Split(body, "\n") {
		info, backticks, isFence := fenceInfoString(line)
		switch {
		case !inFence && isFence:
			inFence, openInfo, openBackticks = true, info, backticks
			current = append(current, line)
			if info == terraformFenceInfo {
				protected = append(protected, strings.Join(current, "\n"))
				current, tfContent = nil, nil
			}
		case inFence && isFence && info == "" && backticks >= openBackticks:
			inFence = false
			if openInfo == terraformFenceInfo {
				tfContents = append(tfContents, strings.Join(tfContent, "\n"))
				tfContent = nil
			}
			current = append(current, line)
		case inFence && openInfo == terraformFenceInfo:
			tfContent = append(tfContent, line)
		default:
			current = append(current, line)
		}
	}

	if inFence && openInfo == terraformFenceInfo {
		tfContents = append(tfContents, strings.Join(tfContent, "\n"))
		current = nil
	}
	return append(protected, strings.Join(current, "\n")), tfContents
}

func fenceDelimiters(body string) (count int, unterminated bool) {
	inFence := false
	openBackticks := 0
	for _, line := range strings.Split(body, "\n") {
		info, backticks, isFence := fenceInfoString(line)
		if !isFence {
			continue
		}
		if inFence {
			if info != "" || backticks < openBackticks {
				continue // an info string, or too short a run, is content rather than a delimiter
			}
			inFence = false
		} else {
			inFence, openBackticks = true, backticks
		}
		count++
	}
	return count, inFence
}

// Reduces a body to the part that must never shrink: the wrapper stripped the way
// upsertComment does it, with block content replaced by a sentinel.
func skeleton(body string, supportsCollapsible bool) string {
	lines := strings.Split(body, "\n")
	if supportsCollapsible && len(lines) > 1 {
		lines = lines[1 : len(lines)-1]
	} else {
		lines = lines[1:]
	}

	protected, tfContents := splitTerraformBlocks(strings.Join(lines, "\n"))
	pieces := make([]string, 0, len(protected)+len(tfContents))
	for i, segment := range protected {
		pieces = append(pieces, segment)
		if i < len(tfContents) {
			pieces = append(pieces, skeletonBlockSentinel)
		}
	}
	return strings.Join(pieces, "\n")
}

// Not usable in the over-prose regime, where fitting the limit and keeping every fence closed
// collide — see TestProtectedProseAloneExceedsLimit.
func assertCommentInvariants(t *testing.T, body string, supportsCollapsible bool) {
	t.Helper()

	assert.True(t, utf8.ValidString(body), "body must be valid utf8")
	assert.LessOrEqual(t, utf8.RuneCountInString(body), GithubCommentMaxLength,
		"body must fit the comment limit")

	delimiters, unterminated := fenceDelimiters(body)
	assert.False(t, unterminated, "every fence must be closed")
	assert.Equal(t, 0, delimiters%2, "``` delimiter count must be even, got %d", delimiters)

	assert.Equal(t, strings.Count(body, "<details"), strings.Count(body, "</details>"),
		"<details> tags must balance")

	if supportsCollapsible {
		assert.True(t, strings.HasSuffix(body, "</details>"),
			"a collapsible body must close its wrapper, otherwise the next edit corrupts it")
	}
}

// Every run of text outside a ```terraform fence must appear verbatim, in order.
func assertProtectedTextSurvives(t *testing.T, body string, formattedReports []string) {
	t.Helper()

	cursor := 0
	for i, report := range formattedReports {
		protected, _ := splitTerraformBlocks(report)
		for j, segment := range protected {
			if segment == "" {
				continue
			}
			offset := strings.Index(body[cursor:], segment)
			if offset < 0 {
				t.Errorf("report %d protected segment %d missing at or after offset %d: %q",
					i, j, cursor, snippet(segment))
				return
			}
			cursor += offset + len(segment)
		}
	}
}

// A trimmed block must be a prefix and a suffix of its original, joined by one elision marker.
// original is always the pristine plan, even for a block trimmed in an earlier round: round
// k+1's head is a prefix of round k's head and so of the original.
func blockHeadPlusTailError(got, original string) error {
	if markers := strings.Count(got, terraformElisionMarker); markers > 1 {
		return fmt.Errorf("block carries %d elision markers, a re-trim must re-emit exactly one", markers)
	} else if markers == 0 {
		if got != original {
			return fmt.Errorf("block shrank without an elision marker: got %d runes of %d",
				utf8.RuneCountInString(got), utf8.RuneCountInString(original))
		}
		return nil
	}

	idx := strings.Index(got, terraformElisionMarker)
	head, tail := got[:idx], got[idx+len(terraformElisionMarker):]

	if !strings.HasPrefix(original, head) {
		return fmt.Errorf("head is not a prefix of the original: %q", snippet(head))
	}
	if !strings.HasSuffix(original, tail) {
		return fmt.Errorf("tail is not a suffix of the original: %q", snippet(tail))
	}
	// The fixtures repeat one resource section, so a prefix and a suffix can both match while
	// jointly claiming more content than the original holds.
	if kept := utf8.RuneCountInString(head) + utf8.RuneCountInString(tail); kept > utf8.RuneCountInString(original) {
		return fmt.Errorf("head+tail keep %d runes of a %d-rune original, so they overlap",
			kept, utf8.RuneCountInString(original))
	}
	return nil
}

func assertBlockIsHeadPlusTail(t *testing.T, got, original string) {
	t.Helper()
	assert.NoError(t, blockHeadPlusTailError(got, original))
}

func TestSplitTerraformBlocks(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantBlocks   []string
		wantProtects []string
	}{
		{
			name:         "no fence",
			body:         "just prose\nover two lines",
			wantBlocks:   nil,
			wantProtects: []string{"just prose\nover two lines"},
		},
		{
			name:         "one terraform fence",
			body:         "before\n```terraform\nplan\nbody\n```\nafter",
			wantBlocks:   []string{"plan\nbody"},
			wantProtects: []string{"before\n```terraform", "```\nafter"},
		},
		{
			name:         "terraform fence plus bash fence",
			body:         "```terraform\nplan\n```\ntext\n```bash\ndigger apply\n```\n",
			wantBlocks:   []string{"plan"},
			wantProtects: []string{"```terraform", "```\ntext\n```bash\ndigger apply\n```\n"},
		},
		{
			name:         "unterminated terraform fence",
			body:         "head\n```terraform\nplan tail lost",
			wantBlocks:   []string{"plan tail lost"},
			wantProtects: []string{"head\n```terraform", ""},
		},
		{
			name:         "nested details",
			body:         "<details><summary>a</summary>\n<details><summary>b</summary>\n```terraform\nplan\n```\n</details>\n</details>",
			wantBlocks:   []string{"plan"},
			wantProtects: []string{"<details><summary>a</summary>\n<details><summary>b</summary>\n```terraform", "```\n</details>\n</details>"},
		},
		{
			name:         "terraform fence indented inside a bash block is bash content",
			body:         "```bash\n  ```terraform\n  not a block\n```\ntail",
			wantBlocks:   nil,
			wantProtects: []string{"```bash\n  ```terraform\n  not a block\n```\ntail"},
		},
		{
			name:         "bare fence inside a plan body closes the block early",
			body:         "```terraform\nplan\n```\nrest of the plan\n```\ntail",
			wantBlocks:   []string{"plan"},
			wantProtects: []string{"```terraform", "```\nrest of the plan\n```\ntail"},
		},
		{
			name:         "two terraform blocks",
			body:         "a\n```terraform\none\n```\nb\n```terraform\ntwo\n```\nc",
			wantBlocks:   []string{"one", "two"},
			wantProtects: []string{"a\n```terraform", "```\nb\n```terraform", "```\nc"},
		},
		{
			name:         "a bare fence inside a longer terraform fence is content",
			body:         "a\n````terraform\nplan\n```\nmore plan\n````\nb",
			wantBlocks:   []string{"plan\n```\nmore plan"},
			wantProtects: []string{"a\n````terraform", "````\nb"},
		},
		{
			name:         "a longer run closes a shorter fence",
			body:         "a\n```terraform\nplan\n`````\nb",
			wantBlocks:   []string{"plan"},
			wantProtects: []string{"a\n```terraform", "`````\nb"},
		},
		{
			name:         "four backticks are a bare fence, not a fence tagged with a backtick",
			body:         "````\nnot terraform\n````\ntail",
			wantBlocks:   nil,
			wantProtects: []string{"````\nnot terraform\n````\ntail"},
		},
		{
			name:         "three spaces of indentation still open a fence",
			body:         "a\n   ```terraform\nplan\n   ```\nb",
			wantBlocks:   []string{"plan"},
			wantProtects: []string{"a\n   ```terraform", "   ```\nb"},
		},
		{
			name:         "four spaces of indentation are an indented code block, not a fence",
			body:         "a\n    ```terraform\nnot a block\n    ```\nb",
			wantBlocks:   nil,
			wantProtects: []string{"a\n    ```terraform\nnot a block\n    ```\nb"},
		},
		{
			name:         "a tab indents a whole column stop, so it is an indented code block too",
			body:         "a\n\t```terraform\nnot a block\n\t```\nb",
			wantBlocks:   nil,
			wantProtects: []string{"a\n\t```terraform\nnot a block\n\t```\nb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protected, blocks := splitTerraformBlocks(tt.body)
			assert.Equal(t, tt.wantBlocks, blocks)
			assert.Equal(t, tt.wantProtects, protected)
			assert.Len(t, protected, len(blocks)+1)

			// The trimmer carries its own parser; it must implement the same grammar as this
			// oracle, or the suite validates it against a grammar it does not use. It reports
			// ok == false on a body that ends inside a fence, where the two deliberately differ.
			if got, ok := splitFencedBody(tt.body); ok {
				assert.Equal(t, tt.wantBlocks, got.blocks, "trimmer disagrees on blocks")
				assert.Equal(t, tt.wantProtects, got.protected, "trimmer disagrees on protected prose")
			}
		})
	}
}

func TestSplitTerraformBlocksOnReferenceFixture(t *testing.T) {
	fixture := readFixture(t, "example_plan.md")
	lines := strings.Split(fixture, "\n")

	protected, blocks := splitTerraformBlocks(fixture)

	require.Len(t, blocks, 1, "the fixture has exactly one terraform block")
	assert.Equal(t, strings.Join(lines[4:42], "\n"), blocks[0])

	delimiters, unterminated := fenceDelimiters(fixture)
	assert.False(t, unterminated)
	assert.Equal(t, 8, delimiters, "four fenced regions: one terraform, three bash")

	require.Len(t, protected, 2)
	assert.True(t, strings.HasSuffix(protected[0], "```terraform"))
	assert.Contains(t, protected[0], "<summary>plan for shared-services")
	assert.True(t, strings.HasPrefix(protected[1], "```\n</details>"))
	assert.Contains(t, protected[1], "digger apply -p shared-services")
	assert.Contains(t, protected[1], "digger apply\n```")
	assert.Contains(t, protected[1], "digger unlock\n```")
	assert.Contains(t, protected[1], "| update   | `module.opentaco.aws_ecs_service.opentaco`")
}

func TestTerraformFenceOutrunsItsContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "fence-free plan output", content: "plan\nbody", want: "```"},
		{name: "empty", content: "", want: "```"},
		{name: "inline code is not a fence", content: "a `inline` b", want: "```"},
		{name: "two backticks cannot close a fence", content: "a\n``\nb", want: "```"},
		{name: "a bare fence", content: "a\n```\nb", want: "````"},
		{name: "an indented fence, as a heredoc renders it", content: "a\n      ```\nb", want: "````"},
		{name: "a tagged fence", content: "a\n```json\nb", want: "````"},
		{name: "the longest run wins", content: "a\n```\nb\n`````\nc\n````\nd", want: "``````"},
		// elideBlock cuts on a rune boundary, so a mid-line run can end up starting a line.
		{name: "a run in the middle of a line", content: "a\nfoo ``` bar\nb", want: "````"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, terraformFence(tt.content))

			// The point of the fence: the content must come back out as one block.
			body := GetTerraformOutputAsCollapsibleComment("s", false)(tt.content)
			split, ok := splitFencedBody(body)
			require.True(t, ok, "the wrapped body must parse")
			require.Len(t, split.blocks, 1)
			assert.Equal(t, tt.content, split.blocks[0])
			assert.Equal(t, body, split.join())
		})
	}
}

func TestSkeletonStripsTheWrapperSymmetricallyWithUpsertComment(t *testing.T) {
	inner := "before\n```terraform\nplan body\n```\nafter"

	collapsible := skeleton(AsCollapsibleComment("t", false)(inner), true)
	assert.NotContains(t, collapsible, "<summary>t</summary>")
	assert.False(t, strings.HasSuffix(collapsible, "</details>"))
	assert.Contains(t, collapsible, skeletonBlockSentinel)
	assert.NotContains(t, collapsible, "plan body")

	plain := skeleton(AsComment("t")(inner), false)
	assert.NotContains(t, plain, "t\nbefore")
	assert.Equal(t, collapsible, plain, "both wrappers must reduce to the same skeleton")
}

func TestSkeletonIgnoresBlockContent(t *testing.T) {
	full := AsCollapsibleComment("t", false)("a\n```terraform\n" + strings.Repeat("x", 5000) + "\n```\nb")
	trimmed := AsCollapsibleComment("t", false)("a\n```terraform\nxx\n```\nb")

	assert.Equal(t, skeleton(full, true), skeleton(trimmed, true))
}

// upsertComment unwraps by dropping the first and last line, so anything else the wrapper adds
// survives the round trip and accumulates on every edit.
func TestWrappersUnwrapExactly(t *testing.T) {
	inner := "before\n```terraform\nplan body\n```\nafter"

	collapsible := strings.Split(AsCollapsibleComment("t", false)(inner), "\n")
	assert.Equal(t, inner, strings.Join(collapsible[1:len(collapsible)-1], "\n"))

	plain := strings.Split(AsComment("t")(inner), "\n")
	assert.Equal(t, inner, strings.Join(plain[1:], "\n"))
}

func TestBlockHeadPlusTail(t *testing.T) {
	original := strings.Repeat("abcdefghij", 200)

	t.Run("legal re-trim of an already elided block", func(t *testing.T) {
		roundK := original[:500] + terraformElisionMarker + original[1500:]
		roundK1 := original[:400] + terraformElisionMarker + original[1600:]

		assert.NoError(t, blockHeadPlusTailError(roundK, original))
		assert.NoError(t, blockHeadPlusTailError(roundK1, original))
	})

	t.Run("untrimmed block must be identical", func(t *testing.T) {
		assert.NoError(t, blockHeadPlusTailError(original, original))
		assert.Error(t, blockHeadPlusTailError(original[:1000], original))
	})

	t.Run("overlapping head and tail", func(t *testing.T) {
		got := original[:1500] + terraformElisionMarker + original[500:]

		assert.True(t, strings.HasPrefix(original, original[:1500]))
		assert.True(t, strings.HasSuffix(original, original[500:]))
		assert.Error(t, blockHeadPlusTailError(got, original), "the length guard must reject an overlap")
	})

	t.Run("nested elision marker", func(t *testing.T) {
		got := original[:500] + terraformElisionMarker + original[600:800] +
			terraformElisionMarker + original[1500:]

		assert.Error(t, blockHeadPlusTailError(got, original))
	})

	t.Run("head not a prefix", func(t *testing.T) {
		assert.Error(t, blockHeadPlusTailError("zzz"+terraformElisionMarker+original[1500:], original))
	})

	t.Run("tail not a suffix", func(t *testing.T) {
		assert.Error(t, blockHeadPlusTailError(original[:500]+terraformElisionMarker+"zzz", original))
	})
}

func TestAssertCommentInvariantsAcceptsTheReferenceFixture(t *testing.T) {
	assertCommentInvariants(t, strings.TrimSuffix(readFixture(t, "example_plan.md"), "\n"), true)
}

func TestAssertProtectedTextSurvivesOnTheReferenceFixture(t *testing.T) {
	fixture := readFixture(t, "example_plan.md")
	lines := strings.Split(fixture, "\n")

	// Feeding the fixture its own plan-output report proves prose is matched around a block,
	// not through it.
	planOutput := GetTerraformOutputAsCollapsibleComment(fixturePlanOutputSummary, false)(strings.Join(lines[4:42], "\n"))

	assertProtectedTextSurvives(t, fixture, []string{planOutput})
}
