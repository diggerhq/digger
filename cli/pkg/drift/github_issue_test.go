package drift

import (
	"strings"
	"testing"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIssueService implements only the issue-related methods the drift
// notification uses; any other call panics via the embedded nil interface.
type mockIssueService struct {
	ci.PullRequestService
	issues    []*ci.Issue
	published map[string]string
	updated   map[int64]string
}

func (m *mockIssueService) ListIssues() ([]*ci.Issue, error) {
	return m.issues, nil
}

func (m *mockIssueService) PublishIssue(title string, body string, labels *[]string) (int64, error) {
	m.published[title] = body
	return 1, nil
}

func (m *mockIssueService) UpdateIssue(ID int64, title string, body string) (int64, error) {
	m.updated[ID] = body
	return ID, nil
}

func newMockIssueService(issues ...*ci.Issue) *mockIssueService {
	return &mockIssueService{
		issues:    issues,
		published: map[string]string{},
		updated:   map[int64]string{},
	}
}

// fencesBalanced counts line-anchored code fences the way GitHub renders them
func fencesBalanced(body string) bool {
	open := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) <= 3 && strings.HasPrefix(trimmed, "```") {
			open = !open
		}
	}
	return !open
}

func TestDriftIssueSmallPlanUnchanged(t *testing.T) {
	mock := newMockIssueService()
	var svc ci.PullRequestService = mock
	ghi := GithubIssueNotification{GithubService: &svc}

	err := ghi.SendNotificationForProject("prod/vpc", "org/repo", "  + resource change")
	require.NoError(t, err)

	body := mock.published["Drift detected in project: prod/vpc"]
	assert.Contains(t, body, "  + resource change")
	assert.NotContains(t, body, "[!WARNING]")
	assert.True(t, fencesBalanced(body))
}

func TestDriftIssueOversizedPlanTruncatedKeepingTail(t *testing.T) {
	mock := newMockIssueService()
	var svc ci.PullRequestService = mock
	ghi := GithubIssueNotification{GithubService: &svc}

	tail := "Plan: 7 to add, 3 to change, 1 to destroy."
	plan := strings.Repeat("  + resource \"aws_instance\" \"drift\" { ami = \"ami-123\" }\n", 2000) + tail // ~110KB

	err := ghi.SendNotificationForProject("prod/vpc", "org/repo", plan)
	require.NoError(t, err)

	body := mock.published["Drift detected in project: prod/vpc"]
	require.NotEmpty(t, body)
	assert.LessOrEqual(t, len(body), 65536, "issue body must fit GitHub's limit")
	assert.True(t, strings.HasSuffix(body, tail+"\n```"), "plan tail with the summary must survive truncation")
	assert.Contains(t, body, "[!WARNING]", "truncation must be announced")
	assert.Truef(t, fencesBalanced(body), "truncated body must keep fences balanced")
}

func TestDriftIssueOversizedPlanOnExistingIssue(t *testing.T) {
	existing := &ci.Issue{ID: 42, Title: "Drift detected in project: prod/vpc", Body: "old"}
	mock := newMockIssueService(existing)
	var svc ci.PullRequestService = mock
	ghi := GithubIssueNotification{GithubService: &svc}

	plan := strings.Repeat("  ~ update line\n", 6000) + "Plan: 1 to change." // ~96KB
	err := ghi.SendNotificationForProject("prod/vpc", "org/repo", plan)
	require.NoError(t, err)

	body := mock.updated[42]
	require.NotEmpty(t, body)
	assert.LessOrEqual(t, len(body), 65536)
	assert.True(t, fencesBalanced(body))
	assert.Empty(t, mock.published, "existing issue must be updated, not duplicated")
}
