package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDriftIssueOversizedPlanLive verifies that a drift notification whose
// plan exceeds GitHub's issue body limit is truncated (tail-preserving,
// fence-balanced) and accepted by the real API. Reruns update the same
// issue, so the scratch repo does not accumulate issues.
func TestDriftIssueOversizedPlanLive(t *testing.T) {
	svc, _ := setupCommentSplittingTest(t)

	tail := "Plan: 7 to add, 3 to change, 1 to destroy."
	plan := strings.Repeat("  + resource \"aws_instance\" \"drift\" { ami = \"ami-0123456789abcdef\" }\n", 2000) + tail // ~130KB

	var prService ci.PullRequestService = &svc
	ghi := drift.GithubIssueNotification{GithubService: &prService}
	projectName := "integration/drift-test"

	err := ghi.SendNotificationForProject(projectName, "", plan)
	require.NoError(t, err, "oversized drift plan must not fail to post")

	title := fmt.Sprintf("Drift detected in project: %v", projectName)
	var found *ci.Issue
	// the list API lags behind issue creation; poll briefly
	for attempt := 0; attempt < 5 && found == nil; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		issues, err := svc.ListIssues()
		require.NoError(t, err)
		for _, issue := range issues {
			if issue.Title == title {
				found = issue
				break
			}
		}
	}
	require.NotNil(t, found, "drift issue must exist")
	assert.LessOrEqual(t, len(found.Body), 65536)
	assert.True(t, strings.HasSuffix(found.Body, tail+"\n```"), "plan summary tail must survive")
	assert.Contains(t, found.Body, "[!WARNING]")
}
