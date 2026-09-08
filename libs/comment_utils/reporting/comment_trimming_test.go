// Bodies in this file all carry a ```terraform fence: only content inside such a fence may
// shrink. Fence-free bodies keep the tail-cut behaviour and are covered in
// reporting_strategy_test.go.
package reporting

import (
	"fmt"
	"html"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type report struct {
	body      string
	formatter func(string) string
}

func (r report) formatted() string { return r.formatter(r.body) }

// Captures the real body and formatter from production instead of duplicating them.
func instructionsReport(t testing.TB, projectName string, supportsMarkdown bool) report {
	t.Helper()
	lazy := NewCiReporterLazy(CiReporter{IsSupportMarkdown: supportsMarkdown})
	require.NoError(t, FormatAndReportExampleCommands(projectName, lazy))
	require.Len(t, lazy.reports, 1)
	return report{body: lazy.reports[0], formatter: lazy.formatters[0]}
}

const fixtureValidationCheckBody = "Terraform plan validation checks succeeded :white_check_mark:"

// upsertComment accumulates only into a comment whose body contains the report title.
const accumulatingTitle = "plan for shared-services"

func planOutputSummary(project string, supportsMarkdown bool) string {
	summary := fixturePlainPlanOutputSummary
	if supportsMarkdown {
		summary = fixturePlanOutputSummary
	}
	return strings.ReplaceAll(summary, "shared-services", project)
}

// The four reports production emits per project (see cli/pkg/digger/digger.go).
func planReports(t testing.TB, project, planOutput string, supportsCollapsible bool) []report {
	t.Helper()

	summary := planOutputSummary(project, supportsCollapsible)
	planFormatter := GetTerraformOutputAsComment(summary)
	wrap := func(title string) func(string) string { return AsComment(title) }
	if supportsCollapsible {
		planFormatter = GetTerraformOutputAsCollapsibleComment(summary, false)
		wrap = func(title string) func(string) string { return AsCollapsibleComment(title, false) }
	}

	return []report{
		{planOutput, planFormatter},
		{fixtureValidationCheckBody, wrap("Terraform plan validation check (" + project + ")")},
		{"\n" + fixturePlanSummaryTable(t), wrap("Plan summary")},
		instructionsReport(t, project, supportsCollapsible),
	}
}

func projectNames(n int) []string {
	projects := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		projects = append(projects, fmt.Sprintf("shared-services-%d", i))
	}
	return projects
}

func replay(t *testing.T, svc MockCiService, strategy ReportStrategy, reports []report, supportsCollapsible bool) []string {
	t.Helper()
	formatted := make([]string, 0, len(reports))
	for _, r := range reports {
		_, _, err := strategy.Report(svc, 1, r.body, r.formatter, supportsCollapsible)
		require.NoError(t, err)
		formatted = append(formatted, r.formatted())
	}
	return formatted
}

func replayLazy(t *testing.T, svc MockCiService, strategy ReportStrategy, reports []report, supportsCollapsible bool) []string {
	t.Helper()
	lazy := NewCiReporterLazy(CiReporter{
		CiService:         svc,
		PrNumber:          1,
		IsSupportMarkdown: supportsCollapsible,
		ReportStrategy:    strategy,
	})
	formatted := make([]string, 0, len(reports))
	for _, r := range reports {
		_, _, err := lazy.Report(r.body, r.formatter)
		require.NoError(t, err)
		formatted = append(formatted, r.formatted())
	}
	_, _, err := lazy.Flush()
	require.NoError(t, err)
	return formatted
}

func assertPresignedPlanLinkSurvives(t *testing.T, body string, supportsMarkdown bool) {
	t.Helper()
	if !supportsMarkdown {
		assert.Contains(t, body, fixturePlainPlanOutputSummary)
		assert.NotContains(t, body, fixtureEscapedPresignedPlanURL)
		return
	}

	assert.Equal(t, fixturePresignedPlanURL, html.UnescapeString(fixtureEscapedPresignedPlanURL))
	assert.Contains(t, body, `href="`+fixtureEscapedPresignedPlanURL+`"`)
	assert.Contains(t, body, "full plan — valid for up to 1 hour")
}

// Measures a run's protected prose by replaying the same reports with empty plan output. It
// grows with the number of plans, so it must be measured rather than hardcoded.
func mandatoryOverhead(t *testing.T, projects []string) int {
	t.Helper()
	svc := newMockCiService()
	strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}
	for _, project := range projects {
		replay(t, svc, strategy, planReports(t, project, "", true), true)
	}
	overhead := utf8.RuneCountInString(onlyBody(t, svc, 1))
	require.LessOrEqual(t, overhead, GithubCommentMaxLength,
		"protected prose alone must fit, otherwise the free budget is negative")
	return overhead
}

func blockFloor() int {
	return minTerraformHeadRunes + utf8.RuneCountInString(terraformElisionMarker) + minTerraformTailRunes
}

func fixturePlanSummaryTable(t testing.TB) string {
	t.Helper()
	lines := strings.Split(readFixture(t, "example_plan.md"), "\n")
	return strings.Join(lines[52:56], "\n") + "\n"
}

// Collapses each terraform block so a failure message shows structure instead of 65k runes.
func planShape(body string) string {
	protected, blocks := splitTerraformBlocks(body)
	var b strings.Builder
	for i, segment := range protected {
		b.WriteString(segment + "\n")
		if i >= len(blocks) {
			continue
		}
		r := []rune(blocks[i])
		if len(r) <= 200 {
			b.WriteString(blocks[i] + "\n")
			continue
		}
		b.WriteString(fmt.Sprintf("%s\n<<< %d runes of block content not shown >>>\n%s\n",
			string(r[:100]), len(r)-200, string(r[len(r)-100:])))
	}
	return b.String()
}

func fixturePlanBlock(t testing.TB) string {
	t.Helper()
	lines := strings.Split(readFixture(t, "example_plan.md"), "\n")
	return strings.Join(lines[4:42], "\n")
}

// Real plan text of exactly n runes. Keeps the *last* n runes of the repeated fixture block so
// the text ends on the block's multi-byte ───── rule.
func planContentOfSize(t testing.TB, n int) string {
	t.Helper()
	require.Positive(t, n)

	block := fixturePlanBlock(t)
	blockRunes := utf8.RuneCountInString(block)

	var repeated strings.Builder
	for written := 0; written < n; {
		if written > 0 {
			repeated.WriteString("\n\n")
			written += 2
		}
		repeated.WriteString(block)
		written += blockRunes
	}
	runes := []rune(repeated.String())
	return string(runes[len(runes)-n:])
}

// A plan output that on its own already exceeds the comment limit. Generated rather than stored:
// a captured plan this size is the reference block repeated some forty times, so the repo would
// carry 65k runes of test data to say one thing.
const oversizedPlanRunes = GithubCommentMaxLength + 100

func oversizedPlanOutput(t testing.TB) string {
	t.Helper()
	return planContentOfSize(t, oversizedPlanRunes)
}

func splitAtElisionMarker(t *testing.T, block string) (head, tail string) {
	t.Helper()
	require.Equal(t, 1, strings.Count(block, terraformElisionMarker),
		"an elided block carries exactly one marker")
	idx := strings.Index(block, terraformElisionMarker)
	return block[:idx], block[idx+len(terraformElisionMarker):]
}

func TestTerraformFenceStructureAlwaysSurvives(t *testing.T) {
	const title = "plan for shared-services"
	planWrapper := GetTerraformOutputAsCollapsibleComment(fixturePlanOutputSummary, false)

	reportTitle := title + " " + fixedTimeOfRun.Format("2006-01-02 15:04:05 (MST)")
	free := GithubCommentMaxLength -
		utf8.RuneCountInString(AsCollapsibleComment(reportTitle, false)("")) -
		utf8.RuneCountInString(planWrapper(""))

	// mustTrim is declared up front, not read off the output: otherwise a trimmer that wrongly
	// elided a small block would pass.
	tests := []struct {
		name     string
		size     int
		mustTrim bool
	}{
		{name: "half the free budget", size: free / 2},
		{name: "an eighth of the free budget", size: free / 8},
		{name: "a hundred runes", size: 100},
		{name: "twice the free budget", size: 2 * free, mustTrim: true},
		{name: "four times the free budget", size: 4 * free, mustTrim: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newMockCiService()
			strategy := CommentPerRunStrategy{Title: title, TimeOfRun: fixedTimeOfRun}
			original := planContentOfSize(t, tt.size)

			_, _, err := strategy.Report(svc, 1, original, planWrapper, true)
			require.NoError(t, err)

			body := onlyBody(t, svc, 1)
			assertCommentInvariants(t, body, true)

			_, blocks := splitTerraformBlocks(body)
			require.Len(t, blocks, 1, "the one terraform block must survive with both delimiters")
			assertBlockIsHeadPlusTail(t, blocks[0], original)

			if !tt.mustTrim {
				assert.Equal(t, original, blocks[0], "a block that fits must not be touched")
				assert.NotContains(t, blocks[0], terraformElisionMarker)
				return
			}

			head, tail := splitAtElisionMarker(t, blocks[0])
			assert.GreaterOrEqual(t, utf8.RuneCountInString(head), minTerraformHeadRunes)
			assert.GreaterOrEqual(t, utf8.RuneCountInString(tail), minTerraformTailRunes)
		})
	}
}

// The four-report accumulation of the reference fixture with a plan output that no longer
// fits: everything except the terraform block must come through verbatim.
func TestReferenceAccumulationKeepsEveryProtectedReport(t *testing.T) {
	svc := newMockCiService()
	strategy := CommentPerRunStrategy{Title: "plan for shared-services", TimeOfRun: fixedTimeOfRun}

	planOutput := oversizedPlanOutput(t)
	require.Greater(t, utf8.RuneCountInString(planOutput), GithubCommentMaxLength,
		"the plan output must not fit, otherwise this test asserts nothing about trimming")

	reports := planReports(t, "shared-services", planOutput, true)

	var body string
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("expected the shape of testdata/example_plan.md with lines 5-42 replaced by head+marker+tail, got:\n%s", planShape(body))
		}
	})

	formatted := replay(t, svc, strategy, reports, true)
	body = onlyBody(t, svc, 1)

	assertCommentInvariants(t, body, true)
	assertProtectedTextSurvives(t, body, formatted)

	for _, want := range []string{
		"<summary>plan for shared-services ",
		`href="` + fixtureEscapedPresignedPlanURL + `">full plan — valid for up to 1 hour</a>`,
		"<details><summary>Terraform plan validation check (shared-services)</summary>",
		fixtureValidationCheckBody,
		"<details><summary>Plan summary</summary>",
		"| update   | `module.opentaco.aws_ecs_service.opentaco`",
		"| recreate | `module.opentaco.aws_ecs_task_definition.opentaco`",
		"<details><summary>Instructions</summary>",
		"digger apply -p shared-services",
		"digger apply\n```",
		"digger unlock\n```",
	} {
		assert.Contains(t, body, want)
	}
	assertPresignedPlanLinkSurvives(t, body, true)
	assert.Equal(t, 3, strings.Count(body, "```bash"), "the three Instructions fences are protected prose")

	_, blocks := splitTerraformBlocks(body)
	require.Len(t, blocks, 1)
	assertBlockIsHeadPlusTail(t, blocks[0], planOutput)
}

// Allocation is greedy and front-first: reserve the floor for every block, then give the whole
// remainder to block 1 until it holds everything it carries, then block 2, and so on. Blocks
// that never get a share sit at exactly the floor.
func TestGreedyAllocationFillsEarliestPlansFirst(t *testing.T) {
	const planSize = 20000
	projects := projectNames(5)

	svc := newMockCiService()
	strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}
	original := planContentOfSize(t, planSize)

	// Grouped per plan, as production emits them: the expected allocation holds only under this
	// ordering.
	var formatted []string
	for _, project := range projects {
		formatted = append(formatted, replay(t, svc, strategy, planReports(t, project, original, true), true)...)
	}
	body := onlyBody(t, svc, 1)

	assertCommentInvariants(t, body, true)
	assertProtectedTextSurvives(t, body, formatted)

	for _, project := range projects {
		assert.Contains(t, body, "digger apply -p "+project)
		assert.Contains(t, body, "<details><summary>Terraform plan validation check ("+project+")</summary>")
	}
	assert.Equal(t, len(projects), strings.Count(body, "<details><summary>Plan summary</summary>"))
	assert.Equal(t, len(projects), strings.Count(body, "| update   | `module.opentaco.aws_ecs_service.opentaco`"))

	_, blocks := splitTerraformBlocks(body)
	require.Len(t, blocks, len(projects))

	floor := blockFloor()
	free := GithubCommentMaxLength - mandatoryOverhead(t, projects)
	wantPartial := free - 2*planSize - 2*floor

	for i, block := range blocks {
		assertBlockIsHeadPlusTail(t, block, original)
		t.Logf("block %d: %d runes", i+1, utf8.RuneCountInString(block))
	}

	assert.Equal(t, original, blocks[0], "block 1 must hold its whole plan")
	assert.Equal(t, original, blocks[1], "block 2 must hold its whole plan")

	head, tail := splitAtElisionMarker(t, blocks[2])
	assert.GreaterOrEqual(t, utf8.RuneCountInString(head), minTerraformHeadRunes)
	assert.GreaterOrEqual(t, utf8.RuneCountInString(tail), minTerraformTailRunes)
	assert.InDelta(t, wantPartial, utf8.RuneCountInString(blocks[2]), 100,
		"block 3 takes what is left after blocks 1 and 2 are whole and blocks 4 and 5 hold the floor")

	assert.Equal(t, floor, utf8.RuneCountInString(blocks[3]), "block 4 gets no share, so it sits at the floor")
	assert.Equal(t, floor, utf8.RuneCountInString(blocks[4]), "block 5 gets no share, so it sits at the floor")

	// Rules out an even split, which would also fit but shred plans that could stay whole.
	evenShare := free / len(projects)
	for i, block := range blocks {
		size := utf8.RuneCountInString(block)
		assert.False(t, size > evenShare-1000 && size < evenShare+1000,
			"block %d must not hold an even share (%d runes) of the budget, got %d", i+1, evenShare, size)
	}
}

// Every plan carries the same payload and a protected pre-signed link. As their combined overhead
// grows, eventually every block can no longer retain the full per-block floor.
func TestManyPlansInOneComment(t *testing.T) {
	tests := []struct {
		name        string
		plans       int
		planSize    int
		floorFits   bool
		mustNotTrim bool
	}{
		{name: "one plan", plans: 1, planSize: 20000, floorFits: true, mustNotTrim: true},
		{name: "small plan", plans: 1, planSize: 200, floorFits: true, mustNotTrim: true},
		{name: "five plans", plans: 5, planSize: 20000, floorFits: true},
		{name: "twenty plans", plans: 20, planSize: 20000, floorFits: true},
		{name: "thirty-one plans", plans: 31, planSize: 20000},
		{name: "thirty-two plans", plans: 32, planSize: 20000},
		{name: "forty plans", plans: 40, planSize: 20000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newMockCiService()
			strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}
			projects := projectNames(tt.plans)
			original := planContentOfSize(t, tt.planSize)

			var formatted []string
			for _, project := range projects {
				formatted = append(formatted, replay(t, svc, strategy, planReports(t, project, original, true), true)...)
			}
			body := onlyBody(t, svc, 1)

			floor := blockFloor()
			free := GithubCommentMaxLength - mandatoryOverhead(t, projects)
			require.Equal(t, tt.floorFits, tt.plans*floor <= free,
				"%d plans need %d runes of floor and have %d free", tt.plans, tt.plans*floor, free)

			assertCommentInvariants(t, body, true)

			_, blocks := splitTerraformBlocks(body)
			assert.Len(t, blocks, tt.plans, "every plan keeps its terraform block, with both delimiters")

			assert.Equal(t, tt.plans, strings.Count(body, "<details><summary>Plan summary</summary>"))
			assert.Equal(t, tt.plans, strings.Count(body, "full plan — valid for up to 1 hour"))
			for _, project := range projects {
				assert.Contains(t, body, "digger apply -p "+project)
			}

			// Past the cliff, how blocks shrink below the floor is deliberately unspecified.
			if !tt.floorFits {
				return
			}

			assertProtectedTextSurvives(t, body, formatted)
			for i, block := range blocks {
				// A plan smaller than the floor is emitted untrimmed.
				assert.GreaterOrEqual(t, utf8.RuneCountInString(block),
					min(utf8.RuneCountInString(original), floor), "block %d", i+1)
			}
			if tt.mustNotTrim {
				for i, block := range blocks {
					assert.Equal(t, original, block, "block %d fits and must not be touched", i+1)
				}
				assert.NotContains(t, body, terraformElisionMarker)
			}
		})
	}
}

// A resource holding markdown puts a bare ``` inside the plan, because plan output renders a
// multi-line string attribute as an indented heredoc. $DIGGER_OUT reaches the same fence with the
// output of any user run step. Before the fence outran its content, the block ended there: the rest
// of the plan became untrimmable prose, so nothing could shrink and the tail cut took every report
// after the plan output with it.
func TestPlanOutputCarryingItsOwnFence(t *testing.T) {
	heredoc := strings.Join([]string{
		`  ~ resource "github_repository_file" "readme" {`,
		`      ~ content = <<-EOT`,
		`            ## Usage`,
		"            ```",
		`            tofu init`,
		"            ```",
		`        EOT`,
		`    }`,
	}, "\n")

	original := heredoc + "\n\n" + planContentOfSize(t, 2*GithubCommentMaxLength)

	for _, collapsible := range []bool{true, false} {
		t.Run(wrapperName(collapsible), func(t *testing.T) {
			svc := newMockCiService()
			strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}

			formatted := replay(t, svc, strategy, planReports(t, "shared-services", original, collapsible), collapsible)
			body := onlyBody(t, svc, 1)

			assertCommentInvariants(t, body, collapsible)
			assertProtectedTextSurvives(t, body, formatted)
			assert.NotContains(t, body, commentTruncationMarker, "the tail cut must not be reached")

			_, blocks := splitTerraformBlocks(body)
			require.Len(t, blocks, 1, "the plan's own fence must stay inside the block")
			assertBlockIsHeadPlusTail(t, blocks[0], original)
			assert.Contains(t, blocks[0], heredoc)

			assert.Contains(t, body, "Plan summary")
			assert.Contains(t, body, "digger apply -p shared-services")
		})
	}
}

// An indented ```terraform in prose is an indented code block, not a fence. Reading it as one hands
// the trimmer a block built from protected prose, which it is then free to shrink; and one left
// unclosed ends the parse inside a fence, which fails the whole body over to the tail cut and takes
// every report after the plan output with it.
func TestIndentedFenceInProseIsNotATrimmableBlock(t *testing.T) {
	// The example has to outgrow the per-block floor. A smaller one is handed back untouched even
	// when the trimmer does read it as a block, so the test would pass against the bug.
	pin := `    module "vpc" { version = "3.1.1" }`
	pinned := strings.TrimSuffix(strings.Repeat(pin+"\n", 2*blockFloor()/utf8.RuneCountInString(pin)), "\n")
	require.Greater(t, utf8.RuneCountInString(pinned), blockFloor())

	tests := []struct {
		name    string
		example string
	}{
		{name: "closed", example: "To pin the module, add:\n\n    ```terraform\n" + pinned + "\n    ```"},
		{name: "unclosed", example: "To pin the module, add:\n\n    ```terraform\n" + pinned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newMockCiService()
			strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}

			reports := planReports(t, "shared-services", oversizedPlanOutput(t), true)
			reports[1].body += "\n" + tt.example

			formatted := replay(t, svc, strategy, reports, true)
			body := onlyBody(t, svc, 1)

			assertCommentInvariants(t, body, true)
			assertProtectedTextSurvives(t, body, formatted)
			assert.NotContains(t, body, commentTruncationMarker, "the tail cut must not be reached")

			_, blocks := splitTerraformBlocks(body)
			require.Len(t, blocks, 1, "only the plan output is a terraform block")
			assert.Contains(t, body, tt.example, "the indented example is prose and survives verbatim")
		})
	}
}

// A plan echoing a previous digger comment carries the elision marker verbatim, so elideBlock
// re-splits at a marker it never wrote — here one a few runes in, leaving almost no head. The
// block must still fill the allocation it was given rather than collapse to a marker plus a tail.
func TestPlanOutputCarryingTheElisionMarkerVerbatim(t *testing.T) {
	const echoedHead = "OpenTofu will perform the following actions:"

	pristine := planContentOfSize(t, 2*GithubCommentMaxLength)

	// Both runs report the same prose, so the block is allocated the same budget in each.
	trimmedBlock := func(t *testing.T, planOutput string) string {
		t.Helper()
		svc := newMockCiService()
		strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}

		formatted := replay(t, svc, strategy, planReports(t, "shared-services", planOutput, true)[:1], true)
		body := onlyBody(t, svc, 1)

		assertCommentInvariants(t, body, true)
		assertProtectedTextSurvives(t, body, formatted)
		_, blocks := splitTerraformBlocks(body)
		require.Len(t, blocks, 1)
		assertBlockIsHeadPlusTail(t, blocks[0], planOutput)
		return blocks[0]
	}

	control := trimmedBlock(t, pristine)
	echoed := trimmedBlock(t, echoedHead+terraformElisionMarker+pristine)

	assert.Equal(t, utf8.RuneCountInString(control), utf8.RuneCountInString(echoed),
		"a marker in the plan text must not cost the block any of its allocation")

	head, tail := splitAtElisionMarker(t, echoed)
	assert.Equal(t, echoedHead, head, "the re-split lands on the literal marker, so the head is short")
	assert.Greater(t, utf8.RuneCountInString(tail), minTerraformTailRunes,
		"the head's unused share must go to the tail")
}

// The first trim of an oversized block keeps exactly minTerraformTailRunes of tail, so a backtick
// run placed on that boundary starts the surviving tail's first line even though it sits mid-line in
// the pristine plan. The fence has to outrun every run in the content, not just the ones a line
// begins with, or the cut manufactures a delimiter the wrapper never accounted for.
func TestCutCannotExposeABacktickRunThatClosesTheFence(t *testing.T) {
	// A bare run on its own line: were the fence only three backticks, this would close it.
	trailing := "```\n" + strings.Repeat("x", minTerraformTailRunes-4)
	require.Equal(t, minTerraformTailRunes, utf8.RuneCountInString(trailing))

	original := planContentOfSize(t, 2*GithubCommentMaxLength) + " inline " + trailing
	lines := strings.Split(original, "\n")
	require.False(t, strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "`"),
		"the run must sit mid-line, otherwise a line-start scan would already catch it")

	svc := newMockCiService()
	strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}
	formatted := replay(t, svc, strategy, planReports(t, "shared-services", original, true), true)
	body := onlyBody(t, svc, 1)

	assertCommentInvariants(t, body, true)
	assertProtectedTextSurvives(t, body, formatted)
	assert.NotContains(t, body, commentTruncationMarker, "the tail cut must not be reached")

	_, blocks := splitTerraformBlocks(body)
	require.Len(t, blocks, 1)
	assertBlockIsHeadPlusTail(t, blocks[0], original)

	_, tail := splitAtElisionMarker(t, blocks[0])
	assert.Equal(t, trailing, tail, "the exposed run must stay inside the block")
	assert.Contains(t, body, "digger apply -p shared-services")
}

func wrapperName(supportsCollapsible bool) string {
	if supportsCollapsible {
		return "collapsible"
	}
	return "plain"
}

func TestTrimmingHoldsAcrossStrategiesAndWrappers(t *testing.T) {
	planOutput := oversizedPlanOutput(t)

	strategies := []struct {
		name string
		// perComment strategies publish one comment per report instead of accumulating.
		perComment bool
		strategy   ReportStrategy
	}{
		{
			name:     "comment per run",
			strategy: CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun},
		},
		{
			// An empty Title falls back to "Digger run report at <time>".
			name:     "comment per run without a title",
			strategy: CommentPerRunStrategy{TimeOfRun: fixedTimeOfRun},
		},
		{
			name:     "latest run",
			strategy: LatestRunCommentStrategy{TimeOfRun: fixedTimeOfRun},
		},
		{
			name:       "multiple comments",
			perComment: true,
			strategy:   MultipleCommentsStrategy{},
		},
	}

	regimes := []struct {
		name  string
		order func([]report) []report
	}{
		{
			name:  "oversized block first",
			order: func(r []report) []report { return r },
		},
		{
			name:  "oversized block last",
			order: func(r []report) []report { return []report{r[1], r[2], r[3], r[0]} },
		},
		{
			name:  "the same oversized block twice",
			order: func(r []report) []report { return []report{r[0], r[0]} },
		},
	}

	for _, s := range strategies {
		for _, collapsible := range []bool{true, false} {
			for _, lazy := range []bool{false, true} {
				for _, regime := range regimes {
					reporterName := "direct"
					if lazy {
						reporterName = "lazy"
					}
					t.Run(fmt.Sprintf("%s/%s/%s/%s", s.name, wrapperName(collapsible), reporterName, regime.name), func(t *testing.T) {
						svc := newMockCiService()
						reports := regime.order(planReports(t, "shared-services", planOutput, collapsible))

						var formatted []string
						if lazy {
							formatted = replayLazy(t, svc, s.strategy, reports, collapsible)
						} else {
							formatted = replay(t, svc, s.strategy, reports, collapsible)
						}

						if !s.perComment {
							body := onlyBody(t, svc, 1)
							assertCommentInvariants(t, body, collapsible)
							assertProtectedTextSurvives(t, body, formatted)
							assertPresignedPlanLinkSurvives(t, body, collapsible)
							return
						}

						// MultipleCommentsStrategy never accumulates: one report per comment.
						comments, err := svc.GetComments(1)
						require.NoError(t, err)
						require.Len(t, comments, len(reports))
						for i, comment := range comments {
							assertCommentInvariants(t, *comment.Body, collapsible)
							assertProtectedTextSurvives(t, *comment.Body, []string{formatted[i]})
							if strings.Contains(formatted[i], fixtureEscapedPresignedPlanURL) ||
								strings.Contains(formatted[i], fixturePresignedPlanURL) {
								assertPresignedPlanLinkSurvives(t, *comment.Body, collapsible)
							}
						}
					})
				}
			}
		}
	}
}

// upsertComment only ever sees the previous *trimmed* body, so every round hands the trimmer a
// block that already carries an elision marker.
func TestRepeatedEditsDoNotErodeSurvivingContent(t *testing.T) {
	const rounds = 20
	planOutput := oversizedPlanOutput(t)

	for _, collapsible := range []bool{true, false} {
		t.Run(wrapperName(collapsible), func(t *testing.T) {
			svc := newMockCiService()
			strategy := CommentPerRunStrategy{Title: accumulatingTitle, TimeOfRun: fixedTimeOfRun}

			replay(t, svc, strategy, planReports(t, "shared-services", planOutput, collapsible)[:1], collapsible)
			previous := skeleton(onlyBody(t, svc, 1), collapsible)

			for round := 1; round <= rounds; round++ {
				title := fmt.Sprintf("Terraform plan validation check (round %d)", round)
				formatter := AsComment(title)
				if collapsible {
					formatter = AsCollapsibleComment(title, false)
				}
				_, _, err := strategy.Report(svc, 1, fixtureValidationCheckBody, formatter, collapsible)
				require.NoError(t, err)

				body := onlyBody(t, svc, 1)
				assertCommentInvariants(t, body, collapsible)

				// Only the skeleton grows purely by append; a full body always ends with
				// </details>.
				current := skeleton(body, collapsible)
				assert.True(t, strings.HasPrefix(current, previous),
					"round %d: protected prose must only ever grow", round)
				previous = current

				_, blocks := splitTerraformBlocks(body)
				require.Len(t, blocks, 1, "round %d", round)
				require.Equal(t, 1, strings.Count(blocks[0], terraformElisionMarker),
					"round %d: the block keeps its one marker and never nests a second", round)

				head, tail := splitAtElisionMarker(t, blocks[0])
				require.GreaterOrEqual(t, utf8.RuneCountInString(head), minTerraformHeadRunes,
					"round %d: the surviving head must not erode below the floor", round)
				require.GreaterOrEqual(t, utf8.RuneCountInString(tail), minTerraformTailRunes,
					"round %d: the surviving tail must not erode below the floor", round)
			}
		})
	}
}
