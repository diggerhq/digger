package reporting

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/stretchr/testify/assert"
)

// mockCiServiceWithLimit wraps MockCiService with a small comment size limit
// so tests do not need to build 64KB reports.
type mockCiServiceWithLimit struct {
	MockCiService
	maxLength int
}

func (m mockCiServiceWithLimit) CommentMaxLength() int {
	return m.maxLength
}

func newMockCiServiceWithLimit(maxLength int) mockCiServiceWithLimit {
	return mockCiServiceWithLimit{
		MockCiService: MockCiService{CommentsPerPr: map[int][]*ci.Comment{}},
		maxLength:     maxLength,
	}
}

func identityFormatter(report string) string {
	return report
}

func TestMultipleCommentsStrategyPublishesSingleCommentWhenUnderLimit(t *testing.T) {
	svc := newMockCiServiceWithLimit(1000)
	strategy := MultipleCommentsStrategy{}

	_, _, err := strategy.Report(svc, 1, "short report", identityFormatter, true)

	assert.NoError(t, err)
	assert.Len(t, svc.CommentsPerPr[1], 1)
	assert.Equal(t, "short report", *svc.CommentsPerPr[1][0].Body)
}

func TestMultipleCommentsStrategySplitsOversizedReport(t *testing.T) {
	svc := newMockCiServiceWithLimit(1000)
	strategy := MultipleCommentsStrategy{}

	report := strings.Repeat("terraform plan line\n", 200) // ~4KB

	commentId, _, err := strategy.Report(svc, 1, report, identityFormatter, true)

	assert.NoError(t, err)
	comments := svc.CommentsPerPr[1]
	assert.Greater(t, len(comments), 1)
	for i, c := range comments {
		assert.LessOrEqualf(t, len(*c.Body), 1000, "comment %d exceeds limit", i)
	}
	// returned id is the last comment, which carries the report's tail
	assert.Equal(t, comments[len(comments)-1].Id, commentId)
}

func TestUpsertAppendsToExistingCommentWhenItFits(t *testing.T) {
	svc := newMockCiServiceWithLimit(10000)
	strategy := CommentPerRunStrategy{Title: "Digger run report", TimeOfRun: time.Now()}

	_, _, err := strategy.Report(svc, 1, "project a plan", identityFormatter, true)
	assert.NoError(t, err)
	_, _, err = strategy.Report(svc, 1, "project b plan", identityFormatter, true)
	assert.NoError(t, err)

	comments := svc.CommentsPerPr[1]
	assert.Len(t, comments, 1)
	assert.Contains(t, *comments[0].Body, "project a plan")
	assert.Contains(t, *comments[0].Body, "project b plan")
}

func TestUpsertOverflowCreatesContinuationAndLeavesExistingCommentIntact(t *testing.T) {
	svc := newMockCiServiceWithLimit(1000)
	strategy := CommentPerRunStrategy{Title: "Digger run report", TimeOfRun: time.Now()}

	_, _, err := strategy.Report(svc, 1, strings.Repeat("project a plan\n", 40), identityFormatter, true) // ~600 chars
	assert.NoError(t, err)
	firstBody := *svc.CommentsPerPr[1][0].Body

	// appending this would exceed the 1000 char limit
	_, _, err = strategy.Report(svc, 1, strings.Repeat("project b plan\n", 40), identityFormatter, true)
	assert.NoError(t, err)

	comments := svc.CommentsPerPr[1]
	assert.Len(t, comments, 2)
	assert.Equal(t, firstBody, *comments[0].Body, "existing comment must be left untouched on overflow")
	assert.Contains(t, *comments[1].Body, "project b plan")
	for i, c := range comments {
		assert.LessOrEqualf(t, len(*c.Body), 1000, "comment %d exceeds limit", i)
	}
}

func TestUpsertAppendsToNewestContinuationComment(t *testing.T) {
	svc := newMockCiServiceWithLimit(1000)
	strategy := CommentPerRunStrategy{Title: "Digger run report", TimeOfRun: time.Now()}

	_, _, err := strategy.Report(svc, 1, strings.Repeat("project a plan\n", 40), identityFormatter, true)
	assert.NoError(t, err)
	_, _, err = strategy.Report(svc, 1, strings.Repeat("project b plan\n", 40), identityFormatter, true)
	assert.NoError(t, err)
	assert.Len(t, svc.CommentsPerPr[1], 2)

	// a small report must append to the newest (continuation) comment, not
	// re-target the full first comment
	_, _, err = strategy.Report(svc, 1, "project c plan", identityFormatter, true)
	assert.NoError(t, err)

	comments := svc.CommentsPerPr[1]
	assert.Len(t, comments, 2)
	assert.NotContains(t, *comments[0].Body, "project c plan")
	assert.Contains(t, *comments[1].Body, "project c plan")
}

func TestUpsertSplitsOversizedReportOnFreshPr(t *testing.T) {
	svc := newMockCiServiceWithLimit(1000)
	strategy := LatestRunCommentStrategy{TimeOfRun: time.Now()}

	report := strings.Repeat("terraform plan line\n", 200) // ~4KB
	_, _, err := strategy.Report(svc, 1, report, identityFormatter, true)
	assert.NoError(t, err)

	comments := svc.CommentsPerPr[1]
	assert.Greater(t, len(comments), 1)
	for i, c := range comments {
		assert.LessOrEqualf(t, len(*c.Body), 1000, "comment %d exceeds limit", i)
		assert.Containsf(t, *c.Body, "Digger latest run report", "comment %d must carry the report title", i)
	}
}

func TestMultipleCommentsStrategyLinksContinuationsToPreviousComment(t *testing.T) {
	svc := newMockCiServiceWithLimit(1200)
	strategy := MultipleCommentsStrategy{}

	report := strings.Repeat("terraform plan line\n", 200) // ~4KB
	_, _, err := strategy.Report(svc, 1, report, identityFormatter, true)
	assert.NoError(t, err)

	comments := svc.CommentsPerPr[1]
	assert.Greater(t, len(comments), 1)
	for i, c := range comments[1:] {
		previousUrl := comments[i].Url
		assert.Containsf(t, *c.Body, "[previous comment]("+previousUrl+")",
			"comment %d must link back to the previous chunk", i+1)
		assert.NotContains(t, *c.Body, prevCommentUrlPlaceholder)
	}
}

func TestFillPreviousCommentUrlFallsBackToPlainText(t *testing.T) {
	chunk := "Continued from [previous comment](" + prevCommentUrlPlaceholder + ").\nrest"
	assert.Equal(t, "Continued from previous comment.\nrest", fillPreviousCommentUrl(chunk, ""))
	assert.Equal(t, "Continued from [previous comment](https://x.test/1).\nrest", fillPreviousCommentUrl(chunk, "https://x.test/1"))
}

func TestUpsertRewrapKeepsMarkdownRenderable(t *testing.T) {
	svc := newMockCiServiceWithLimit(100000)
	strategy := CommentPerRunStrategy{Title: "Digger run report", TimeOfRun: time.Now()}

	for i := 0; i < 3; i++ {
		report := fmt.Sprintf("report %d with [a link](https://x.test/%d)", i, i)
		_, _, err := strategy.Report(svc, 1, report, identityFormatter, true)
		assert.NoError(t, err)
	}

	body := *svc.CommentsPerPr[1][0].Body
	lines := strings.Split(body, "\n")
	// a blank line after <summary> is required for markdown (links, bold,
	// fences) to render inside the details block
	assert.True(t, strings.HasPrefix(lines[0], "<details"))
	assert.Equal(t, "", lines[1])
	// re-wrapping on append must not accumulate indentation: four leading
	// spaces turn content into a markdown code block
	for _, line := range lines {
		assert.Falsef(t, strings.HasPrefix(line, "    "), "line gained code-block indentation: %q", line)
	}
}

func TestCommentMaxSizeFallsBackToDefault(t *testing.T) {
	plain := MockCiService{CommentsPerPr: map[int][]*ci.Comment{}}
	assert.Equal(t, defaultCommentMaxSize, CommentMaxSize(plain))
	assert.Equal(t, 1000, CommentMaxSize(newMockCiServiceWithLimit(1000)))
}
