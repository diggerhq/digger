package integration

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/libs/ci/bitbucket"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bitbucket twin of the GitHub comment splitting tests. Bitbucket Cloud's
// 32,768-character limit is small enough to hit genuinely, so no size
// override is needed — an ~80KB report forces a real split against the real
// limit. Digger treats Bitbucket as not supporting markdown, so these tests
// exercise the plain (non-collapsible) formatting path.
//
// Required environment variables (tests are skipped when unset):
//
//	DIGGER_INTEGRATION_BITBUCKET_TOKEN      repository access token (Bearer)
//	DIGGER_INTEGRATION_BITBUCKET_WORKSPACE  workspace slug
//	DIGGER_INTEGRATION_BITBUCKET_REPO       repository name
//	DIGGER_INTEGRATION_BITBUCKET_PR         an open PR id in that repository
func setupBitbucketSplittingTest(t *testing.T) (bitbucket.BitbucketAPI, int) {
	SkipCI(t)

	token := os.Getenv("DIGGER_INTEGRATION_BITBUCKET_TOKEN")
	workspace := os.Getenv("DIGGER_INTEGRATION_BITBUCKET_WORKSPACE")
	repo := os.Getenv("DIGGER_INTEGRATION_BITBUCKET_REPO")
	prStr := os.Getenv("DIGGER_INTEGRATION_BITBUCKET_PR")
	if token == "" || workspace == "" || repo == "" || prStr == "" {
		t.Skip("Skipping: DIGGER_INTEGRATION_BITBUCKET_TOKEN, DIGGER_INTEGRATION_BITBUCKET_WORKSPACE, DIGGER_INTEGRATION_BITBUCKET_REPO and DIGGER_INTEGRATION_BITBUCKET_PR must be set")
	}
	prNumber, err := strconv.Atoi(prStr)
	require.NoError(t, err)

	return bitbucket.BitbucketAPI{
		AuthToken:     token,
		RepoWorkspace: workspace,
		RepoName:      repo,
	}, prNumber
}

const bitbucketCommentLimit = 32768

func TestBitbucketCommentSplittingMultipleComments(t *testing.T) {
	svc, prNumber := setupBitbucketSplittingTest(t)

	marker := fmt.Sprintf("digger-1645-bb-multi-%d", time.Now().UnixNano())

	reporter := reporting.CiReporter{
		CiService:         svc,
		PrNumber:          prNumber,
		IsSupportMarkdown: false,
		ReportStrategy:    reporting.MultipleCommentsStrategy{},
	}

	plan := fakePlanOutput(marker, 80_000) // ~2.5x the limit
	_, lastUrl, err := reporter.Report(plan, reporting.GetTerraformOutputAsComment("Plan output"))
	require.NoError(t, err, "posting an oversized plan must not fail on Bitbucket")
	assert.NotEmpty(t, lastUrl, "Bitbucket must return the comment url")

	comments, err := svc.GetComments(prNumber)
	require.NoError(t, err)

	var created []string
	for _, c := range comments {
		if c.Body != nil && strings.Contains(*c.Body, marker) {
			created = append(created, *c.Body)
		}
	}

	require.GreaterOrEqual(t, len(created), 3, "80KB plan must be split into at least 3 comments")
	for i, body := range created {
		assert.LessOrEqualf(t, len(body), bitbucketCommentLimit, "comment %d exceeds Bitbucket's limit", i)
	}
	assert.Contains(t, created[len(created)-1], fmt.Sprintf("Plan: 1 to add, 0 to change, 0 to destroy. [%s]", marker))
	for i, body := range created[1:] {
		assert.Containsf(t, body, "[previous comment](https://bitbucket.org/", "comment %d must link back to the previous chunk", i+1)
	}
}

func TestBitbucketCommentSplittingPerRunOverflow(t *testing.T) {
	svc, prNumber := setupBitbucketSplittingTest(t)

	marker := fmt.Sprintf("digger-1645-bb-upsert-%d", time.Now().UnixNano())
	title := "Digger run report " + marker
	strategy := reporting.CommentPerRunStrategy{Title: title, TimeOfRun: time.Now()}
	reporter := reporting.CiReporter{
		CiService:         svc,
		PrNumber:          prNumber,
		IsSupportMarkdown: false,
		ReportStrategy:    strategy,
	}
	formatter := reporting.GetTerraformOutputAsComment("Plan output")

	countTitled := func() []string {
		comments, err := svc.GetComments(prNumber)
		require.NoError(t, err)
		var bodies []string
		for _, c := range comments {
			if c.Body != nil && strings.Contains(*c.Body, title) {
				bodies = append(bodies, *c.Body)
			}
		}
		return bodies
	}

	// project a fills most of the shared comment
	_, _, err := reporter.Report(fakePlanOutput(marker+"-project-a", 18_000), formatter)
	require.NoError(t, err)
	bodies := countTitled()
	require.Len(t, bodies, 1)

	// project b does not fit: the shared comment must be left intact and a
	// continuation created instead of failing
	_, _, err = reporter.Report(fakePlanOutput(marker+"-project-b", 18_000), formatter)
	require.NoError(t, err, "overflowing the shared per-run comment must not fail")

	bodies = countTitled()
	require.Len(t, bodies, 2, "overflow must create a continuation comment")
	assert.Contains(t, bodies[0], marker+"-project-a")
	assert.NotContains(t, bodies[0], marker+"-project-b", "existing comment must be left untouched")
	assert.Contains(t, bodies[1], marker+"-project-b")
	for i, body := range bodies {
		assert.LessOrEqualf(t, len(body), bitbucketCommentLimit, "comment %d exceeds Bitbucket's limit", i)
	}

	// project c is small: it must append to the newest continuation comment
	_, _, err = reporter.Report("small plan for "+marker+"-project-c", formatter)
	require.NoError(t, err)

	bodies = countTitled()
	require.Len(t, bodies, 2, "small report must append, not create another comment")
	assert.NotContains(t, bodies[0], marker+"-project-c")
	assert.Contains(t, bodies[1], marker+"-project-c")
}
