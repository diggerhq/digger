package locking

import (
	"fmt"

	"github.com/diggerhq/digger/libs/scheduler"
)

// PerformLockingActionForProjects runs command's lock action on every lock.
// Plan, apply and lock take locks all or nothing: every project is checked
// before any is taken, so a PR never holds part of what it needs while
// another PR holds the rest. Callers make the whole call atomic, for
// example by running it in one database transaction, and keep two PRs in
// the same repository from running it at once.
func PerformLockingActionForProjects(locks []PullRequestLock, command scheduler.DiggerCommand) error {
	switch command {
	case scheduler.DiggerCommandPlan, scheduler.DiggerCommandApply, scheduler.DiggerCommandLock:
		var free []PullRequestLock
		for _, l := range locks {
			holder, err := l.currentHolder()
			if err != nil {
				return fmt.Errorf("project %s: failed to lock project: %w", l.ProjectName, err)
			}
			if holder == nil {
				free = append(free, l)
			}
		}
		for _, l := range free {
			if _, err := l.InternalLock.Lock(l.PrNumber, l.LockId()); err != nil {
				return fmt.Errorf("project %s: failed to lock project: %w", l.ProjectName, err)
			}
		}
	case scheduler.DiggerCommandUnlock:
		for _, l := range locks {
			if _, err := l.Unlock(); err != nil {
				return fmt.Errorf("project %s: failed to unlock project: %w", l.ProjectName, err)
			}
		}
	}
	return nil
}
