package integration

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	dg_github "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise comment splitting (issue #1645) against the real
// GitHub API, which enforces the 65,536-character comment body limit that
// unit tests cannot reproduce.
//
// Required environment variables (the tests are skipped when unset):
//
//	GITHUB_TOKEN                   token with write access to the test repo
//	DIGGER_INTEGRATION_REPO_OWNER  e.g. "my-org"
//	DIGGER_INTEGRATION_REPO_NAME   a scratch repository name
//	DIGGER_INTEGRATION_PR          an open PR number in that repository
func setupCommentSplittingTest(t *testing.T) (dg_github.GithubService, int) {
	SkipCI(t)

	ghToken := os.Getenv("GITHUB_TOKEN")
	owner := os.Getenv("DIGGER_INTEGRATION_REPO_OWNER")
	repo := os.Getenv("DIGGER_INTEGRATION_REPO_NAME")
	prStr := os.Getenv("DIGGER_INTEGRATION_PR")
	if ghToken == "" || owner == "" || repo == "" || prStr == "" {
		t.Skip("Skipping: GITHUB_TOKEN, DIGGER_INTEGRATION_REPO_OWNER, DIGGER_INTEGRATION_REPO_NAME and DIGGER_INTEGRATION_PR must be set")
	}
	prNumber, err := strconv.Atoi(prStr)
	require.NoError(t, err)

	svc, err := dg_github.GithubServiceProviderBasic{}.NewService(ghToken, repo, owner)
	require.NoError(t, err)
	return svc, prNumber
}

// deleteCommentsContaining removes all PR comments carrying marker, so
// reruns start from a clean slate and the scratch PR does not accumulate
// megabytes of test output.
func deleteCommentsContaining(t *testing.T, svc dg_github.GithubService, prNumber int, marker string) {
	comments, err := svc.GetComments(prNumber)
	if err != nil {
		t.Logf("cleanup: could not list comments: %v", err)
		return
	}
	for _, c := range comments {
		if c.Body != nil && strings.Contains(*c.Body, marker) {
			if err := svc.DeleteComment(c.Id); err != nil {
				t.Logf("cleanup: could not delete comment %v: %v", c.Id, err)
			}
		}
	}
}

// fakePlanOutput builds a synthetic terraform plan of roughly targetSize
// bytes, tagged with marker on every line and ending with a recognizable
// plan summary.
func fakePlanOutput(marker string, targetSize int) string {
	line := fmt.Sprintf("  + resource \"aws_instance\" \"%s\" { ami = \"ami-0123456789abcdef\" }\n", marker)
	var sb strings.Builder
	for sb.Len() < targetSize {
		sb.WriteString(line)
	}
	fmt.Fprintf(&sb, "Plan: 1 to add, 0 to change, 0 to destroy. [%s]", marker)
	return sb.String()
}

const githubCommentLimit = 65536

// TestCommentSplittingMultipleCommentsStrategy reproduces the exact failure
// from issue #1645: a single plan larger than the GitHub comment limit posted
// through the default (multiple_comments) strategy, formatted the same way
// cli/pkg/digger reports plan output.
func TestCommentSplittingMultipleCommentsStrategy(t *testing.T) {
	svc, prNumber := setupCommentSplittingTest(t)

	marker := fmt.Sprintf("digger-1645-multi-%d", time.Now().UnixNano())
	defer deleteCommentsContaining(t, svc, prNumber, marker)

	reporter := reporting.CiReporter{
		CiService:         &svc,
		PrNumber:          prNumber,
		IsSupportMarkdown: true,
		ReportStrategy:    reporting.MultipleCommentsStrategy{},
	}

	plan := fakePlanOutput(marker, 150_000) // ~2.3x the limit
	commentId, commentUrl, err := reporter.Report(plan, reporting.GetTerraformOutputAsCollapsibleComment("Plan output", true))
	require.NoError(t, err, "posting an oversized plan must not fail with 422")
	assert.NotEmpty(t, commentId)
	assert.NotEmpty(t, commentUrl)

	comments, err := svc.GetComments(prNumber)
	require.NoError(t, err)

	var created []string
	var createdUrls []string
	for _, c := range comments {
		if c.Body != nil && strings.Contains(*c.Body, marker) {
			created = append(created, *c.Body)
			createdUrls = append(createdUrls, c.Url)
		}
	}

	assert.GreaterOrEqual(t, len(created), 3, "150KB plan must be split into at least 3 comments")
	for i, body := range created {
		assert.LessOrEqualf(t, len(body), githubCommentLimit, "comment %d exceeds GitHub's limit", i)
	}
	// the tail with the plan summary must survive in the last chunk
	assert.Contains(t, created[len(created)-1], fmt.Sprintf("Plan: 1 to add, 0 to change, 0 to destroy. [%s]", marker))
	// continuation chunks must link back to the previous chunk's comment
	for i, body := range created[1:] {
		assert.Containsf(t, body, fmt.Sprintf("Continued from [previous comment](%s).", createdUrls[i]),
			"comment %d must link back to the previous chunk", i+1)
	}
}

// TestCommentSplittingCommentPerRunOverflow reproduces the append-overflow
// failure: many projects reporting into a single per-run comment until it
// cannot hold the next report.
func TestCommentSplittingCommentPerRunOverflow(t *testing.T) {
	svc, prNumber := setupCommentSplittingTest(t)

	marker := fmt.Sprintf("digger-1645-upsert-%d", time.Now().UnixNano())
	defer deleteCommentsContaining(t, svc, prNumber, marker)

	title := "Digger run report " + marker
	strategy := reporting.CommentPerRunStrategy{Title: title, TimeOfRun: time.Now()}
	reporter := reporting.CiReporter{
		CiService:         &svc,
		PrNumber:          prNumber,
		IsSupportMarkdown: true,
		ReportStrategy:    strategy,
	}
	formatter := reporting.GetTerraformOutputAsCollapsibleComment("Plan output", false)

	countTitled := func() ([]string, []string) {
		comments, err := svc.GetComments(prNumber)
		require.NoError(t, err)
		var ids, bodies []string
		for _, c := range comments {
			if c.Body != nil && strings.Contains(*c.Body, title) {
				ids = append(ids, c.Id)
				bodies = append(bodies, *c.Body)
			}
		}
		return ids, bodies
	}

	// project a fills most of the shared comment
	_, _, err := reporter.Report(fakePlanOutput(marker+"-project-a", 40_000), formatter)
	require.NoError(t, err)
	_, bodies := countTitled()
	require.Len(t, bodies, 1)

	// project b does not fit any more: before the fix this failed with
	// "422 Validation Failed: body is too long"
	_, _, err = reporter.Report(fakePlanOutput(marker+"-project-b", 40_000), formatter)
	require.NoError(t, err, "overflowing the shared per-run comment must not fail with 422")

	_, bodies = countTitled()
	require.Len(t, bodies, 2, "overflow must create a continuation comment")
	assert.Contains(t, bodies[0], marker+"-project-a")
	assert.NotContains(t, bodies[0], marker+"-project-b", "existing comment must be left untouched")
	assert.Contains(t, bodies[1], marker+"-project-b")
	for i, body := range bodies {
		assert.LessOrEqualf(t, len(body), githubCommentLimit, "comment %d exceeds GitHub's limit", i)
	}

	// project c is small: it must append to the newest continuation comment
	_, _, err = reporter.Report("small plan for "+marker+"-project-c", formatter)
	require.NoError(t, err)

	_, bodies = countTitled()
	require.Len(t, bodies, 2, "small report must append, not create another comment")
	assert.NotContains(t, bodies[0], marker+"-project-c")
	assert.Contains(t, bodies[1], marker+"-project-c")
}
