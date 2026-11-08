package locking

import (
	"testing"

	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closedPRs struct {
	*orchestrator.MockGithubPullrequestManager
	closed map[int]bool
}

func (c closedPRs) IsClosed(prNumber int) (bool, error) { return c.closed[prNumber], nil }

func prLocks(internal Lock, ciService closedPRs, pr int, projects ...string) []PullRequestLock {
	var locks []PullRequestLock
	for _, p := range projects {
		locks = append(locks, PullRequestLock{
			InternalLock:     internal,
			CIService:        ciService,
			Reporter:         &reporting.MockReporter{},
			ProjectName:      p,
			ProjectNamespace: "o/r",
			PrNumber:         pr,
		})
	}
	return locks
}

func TestPerformLockingActionForProjects(t *testing.T) {
	ciService := closedPRs{&orchestrator.MockGithubPullrequestManager{}, map[int]bool{3: true}}
	plan := scheduler.DiggerCommandPlan

	t.Run("takes none when another PR holds one", func(t *testing.T) {
		lock := &MockLock{MapLock: map[string]int{"o/r#b": 2}}
		err := PerformLockingActionForProjects(prLocks(lock, ciService, 1, "a", "b"), plan)
		require.ErrorIs(t, err, ErrLockedByAnotherPR)
		assert.Contains(t, err.Error(), "project b:")
		assert.Contains(t, err.Error(), "Project o/r#b locked by another PR #2(failed to acquire lock b)", "Infrabot parses this text")
		assert.Equal(t, map[string]int{"o/r#b": 2}, lock.MapLock, "a must stay free")
	})

	t.Run("keeps its own locks and takes the rest", func(t *testing.T) {
		lock := &MockLock{MapLock: map[string]int{"o/r#a": 1}}
		err := PerformLockingActionForProjects(prLocks(lock, ciService, 1, "a", "b"), plan)
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"o/r#a": 1, "o/r#b": 1}, lock.MapLock)
	})

	t.Run("clears a closed PR's lock", func(t *testing.T) {
		lock := &MockLock{MapLock: map[string]int{"o/r#a": 3}}
		err := PerformLockingActionForProjects(prLocks(lock, ciService, 1, "a"), plan)
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"o/r#a": 1}, lock.MapLock)
	})

	t.Run("unlock releases every project", func(t *testing.T) {
		lock := &MockLock{MapLock: map[string]int{"o/r#a": 1, "o/r#b": 1}}
		err := PerformLockingActionForProjects(prLocks(lock, ciService, 1, "a", "b"), scheduler.DiggerCommandUnlock)
		require.NoError(t, err)
		assert.Empty(t, lock.MapLock)
	})
}
