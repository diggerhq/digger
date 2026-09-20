package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	dg_gitlab "github.com/diggerhq/digger/libs/ci/gitlab"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitLab twin of the GitHub comment splitting tests. GitLab's real limit is
// 1,000,000 characters, so instead of posting megabytes the service is
// wrapped with a small CommentMaxLength override to exercise the split
// mechanics, the discussions API and backlink URL construction against the
// real API.
//
// Required environment variables (tests are skipped when unset):
//
//	GITLAB_TOKEN                            token with api scope
//	DIGGER_INTEGRATION_GITLAB_PROJECT_ID    numeric project id
//	DIGGER_INTEGRATION_GITLAB_MR_IID        an open MR iid in that project
//	DIGGER_INTEGRATION_GITLAB_NAMESPACE     project namespace (group/user path)
//	DIGGER_INTEGRATION_GITLAB_PROJECT_NAME  project path name
func setupGitlabSplittingTest(t *testing.T) (*dg_gitlab.GitLabService, int, string) {
	SkipCI(t)

	token := os.Getenv("GITLAB_TOKEN")
	projectIdStr := os.Getenv("DIGGER_INTEGRATION_GITLAB_PROJECT_ID")
	mrIidStr := os.Getenv("DIGGER_INTEGRATION_GITLAB_MR_IID")
	namespace := os.Getenv("DIGGER_INTEGRATION_GITLAB_NAMESPACE")
	projectName := os.Getenv("DIGGER_INTEGRATION_GITLAB_PROJECT_NAME")
	if token == "" || projectIdStr == "" || mrIidStr == "" || namespace == "" || projectName == "" {
		t.Skip("Skipping: GITLAB_TOKEN, DIGGER_INTEGRATION_GITLAB_PROJECT_ID, DIGGER_INTEGRATION_GITLAB_MR_IID, DIGGER_INTEGRATION_GITLAB_NAMESPACE and DIGGER_INTEGRATION_GITLAB_PROJECT_NAME must be set")
	}

	projectId, err := strconv.Atoi(projectIdStr)
	require.NoError(t, err)
	mrIid, err := strconv.Atoi(mrIidStr)
	require.NoError(t, err)

	ctx := &dg_gitlab.GitLabContext{
		ProjectId:        &projectId,
		MergeRequestIId:  &mrIid,
		ProjectName:      projectName,
		ProjectNamespace: namespace,
		Token:            token,
	}
	svc, err := dg_gitlab.NewGitLabService(token, ctx, "")
	require.NoError(t, err)
	return svc, mrIid, token
}

// gitlabServiceWithLimit lowers the comment size limit so tests can force
// splits without posting megabytes to gitlab.com.
type gitlabServiceWithLimit struct {
	*dg_gitlab.GitLabService
	maxLength int
}

func (m gitlabServiceWithLimit) CommentMaxLength() int {
	return m.maxLength
}

type gitlabNote struct {
	Id     int    `json:"id"`
	Body   string `json:"body"`
	System bool   `json:"system"`
}

// listMrNotes fetches MR notes via the raw API (GitLabService.GetComments is
// a stub), oldest first.
func listMrNotes(t *testing.T, token string, projectId string, mrIid int) []gitlabNote {
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/merge_requests/%d/notes?per_page=100&order_by=created_at&sort=asc", projectId, mrIid)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var notes []gitlabNote
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&notes))
	return notes
}

var notePrevLinkRegex = regexp.MustCompile(`\[previous comment\]\((\S+)#note_(\d+)\)`)

func TestGitlabCommentSplittingMultipleComments(t *testing.T) {
	svc, mrIid, token := setupGitlabSplittingTest(t)
	limited := gitlabServiceWithLimit{GitLabService: svc, maxLength: 2500}

	marker := fmt.Sprintf("digger-1645-gitlab-%d", time.Now().UnixNano())

	reporter := reporting.CiReporter{
		CiService:         limited,
		PrNumber:          mrIid,
		IsSupportMarkdown: true,
		ReportStrategy:    reporting.MultipleCommentsStrategy{},
	}

	plan := fakePlanOutput(marker, 8_000)
	_, lastUrl, err := reporter.Report(plan, reporting.GetTerraformOutputAsCollapsibleComment("Plan output", true))
	require.NoError(t, err)
	assert.Contains(t, lastUrl, "#note_", "returned url must be a note anchor")

	var created []gitlabNote
	for _, n := range listMrNotes(t, token, os.Getenv("DIGGER_INTEGRATION_GITLAB_PROJECT_ID"), mrIid) {
		if !n.System && strings.Contains(n.Body, marker) {
			created = append(created, n)
		}
	}

	require.GreaterOrEqual(t, len(created), 3, "8KB plan with a 2.5KB limit must be split into at least 3 notes")
	for i, n := range created {
		assert.LessOrEqualf(t, len(n.Body), 2500, "note %d exceeds the limit", i)
	}
	// summary tail survives in the last note
	assert.Contains(t, created[len(created)-1].Body, fmt.Sprintf("Plan: 1 to add, 0 to change, 0 to destroy. [%s]", marker))

	// continuation notes must link back to the actual previous note
	expectedUrlPrefix := fmt.Sprintf("https://gitlab.com/%s/%s/-/merge_requests/%d",
		os.Getenv("DIGGER_INTEGRATION_GITLAB_NAMESPACE"), os.Getenv("DIGGER_INTEGRATION_GITLAB_PROJECT_NAME"), mrIid)
	for i, n := range created[1:] {
		m := notePrevLinkRegex.FindStringSubmatch(n.Body)
		require.NotNilf(t, m, "note %d must contain a previous-comment link", i+1)
		assert.Equalf(t, expectedUrlPrefix, m[1], "note %d link must point at this MR", i+1)
		linkedId, err := strconv.Atoi(m[2])
		require.NoError(t, err)
		assert.Equalf(t, created[i].Id, linkedId, "note %d must link to its predecessor", i+1)
	}
}

func TestGitlabCommentSplittingFreshOversizedPerRunReport(t *testing.T) {
	svc, mrIid, token := setupGitlabSplittingTest(t)
	limited := gitlabServiceWithLimit{GitLabService: svc, maxLength: 2500}

	marker := fmt.Sprintf("digger-1645-gitlab-upsert-%d", time.Now().UnixNano())
	title := "Digger run report " + marker
	strategy := reporting.CommentPerRunStrategy{Title: title, TimeOfRun: time.Now()}
	reporter := reporting.CiReporter{
		CiService:         limited,
		PrNumber:          mrIid,
		IsSupportMarkdown: true,
		ReportStrategy:    strategy,
	}

	// note: GitLabService.GetComments is currently a stub returning nil, so
	// on GitLab the per-run strategies always create fresh comments; this
	// verifies the oversized fresh report splits instead of failing
	_, _, err := reporter.Report(fakePlanOutput(marker, 8_000), reporting.GetTerraformOutputAsCollapsibleComment("Plan output", false))
	require.NoError(t, err)

	count := 0
	for _, n := range listMrNotes(t, token, os.Getenv("DIGGER_INTEGRATION_GITLAB_PROJECT_ID"), mrIid) {
		if !n.System && strings.Contains(n.Body, marker) {
			count++
			assert.Contains(t, n.Body, title, "every chunk must carry the report title")
			assert.LessOrEqual(t, len(n.Body), 2500)
		}
	}
	require.GreaterOrEqual(t, count, 3)
}
