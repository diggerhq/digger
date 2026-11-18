package github

import (
	"testing"

	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/assert"
)

const (
	diggerAppID  int64 = 1
	actionsAppID int64 = 2
)

func checkRun(name, status, conclusion, detailsURL string, appID int64) *github.CheckRun {
	return &github.CheckRun{
		Name:       github.String(name),
		Status:     github.String(status),
		Conclusion: github.String(conclusion),
		DetailsURL: github.String(detailsURL),
		App:        &github.App{ID: github.Int64(appID)},
	}
}

// diggerApplyInProgress mirrors the check runs seen on a PR while the digger apply job is running,
// after earlier apply attempts on the same commit have failed.
func diggerApplyInProgress() []*github.CheckRun {
	return []*github.CheckRun{
		checkRun("Quality Checks", "completed", "success", "https://github.com/o/r/actions/runs/100/job/1", actionsAppID),
		checkRun("digger/plan", "completed", "success", "https://digger.example", diggerAppID),
		checkRun("platform-clickhouse/plan", "completed", "success", "https://digger.example", diggerAppID),
		checkRun("Digger", "completed", "failure", "https://github.com/o/r/actions/runs/200/job/2", actionsAppID),
		checkRun("digger/apply", "in_progress", "", "https://digger.example", diggerAppID),
		checkRun("platform-clickhouse/apply", "in_progress", "", "https://digger.example", diggerAppID),
		checkRun("Digger", "in_progress", "", "https://github.com/o/r/actions/runs/300/job/3", actionsAppID),
	}
}

func TestOnlyDiggerApplyChecksPendingIgnoresDiggerOwnChecks(t *testing.T) {
	assert.True(t, onlyDiggerApplyChecksPending(diggerApplyInProgress(), "300"))
}

func TestOnlyDiggerApplyChecksPendingWithoutRunIDKeepsActionsJob(t *testing.T) {
	assert.False(t, onlyDiggerApplyChecksPending(diggerApplyInProgress(), ""))
}

func TestOnlyDiggerApplyChecksPendingBlockedByOtherCheck(t *testing.T) {
	checkRuns := append(diggerApplyInProgress(),
		checkRun("security-scan", "completed", "failure", "https://github.com/o/r/actions/runs/400/job/4", actionsAppID))

	assert.False(t, onlyDiggerApplyChecksPending(checkRuns, "300"))
}

func TestOnlyDiggerApplyChecksPendingKeepsApplySuffixFromOtherApp(t *testing.T) {
	checkRuns := []*github.CheckRun{
		checkRun("digger/apply", "in_progress", "", "https://digger.example", diggerAppID),
		checkRun("other-tool/apply", "in_progress", "", "https://other.example", actionsAppID),
	}

	assert.False(t, onlyDiggerApplyChecksPending(checkRuns, ""))
}
