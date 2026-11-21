package locking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
)

// How long a PR waits for another PR in the repository to finish with its
// locks, and how long Postgres lets a holder sit idle mid-way.
const repositoryLockTimeout = time.Minute

// PerformPRLockingAction runs command's lock action on the PR's projects when
// config enables PR locks. Taking locks is all or nothing (see
// locking.PerformLockingActionForProjects), in one transaction that holds a
// per-repository lock, so two PRs can't each take part of the same set.
// Report a failure with LockFailureComment.
func PerformPRLockingAction(config *digger_config.DiggerConfig, orgId uint, ciService ci.PullRequestService, repoFullName string, prNumber int, projects []digger_config.Project, command scheduler.DiggerCommand) error {
	if !config.PrLocks {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), repositoryLockTimeout)
	defer cancel()
	return models.DB.WithRepositoryPRLocks(ctx, repoFullName, repositoryLockTimeout, func(tx *models.Database) error {
		locks := make([]locking.PullRequestLock, 0, len(projects))
		for _, project := range projects {
			locks = append(locks, locking.PullRequestLock{
				InternalLock:     BackendDBLock{OrgId: orgId, DB: tx},
				CIService:        ciService,
				Reporter:         reporting.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         prNumber,
			})
		}
		return locking.PerformLockingActionForProjects(locks, command)
	})
}

// LockFailureComment is the PR comment for an error from
// PerformPRLockingAction. A project held by another PR keeps the wording
// that names the holder. Anything else gets a retry note, worded so it isn't
// mistaken for a lock held by another PR.
func LockFailureComment(err error) string {
	switch {
	case errors.Is(err, locking.ErrLockedByAnotherPR):
		return fmt.Sprintf(":x: Failed perform lock action on %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return ":hourglass: Another PR in this repository is still taking its project locks, so this request was skipped. Try again shortly."
	}
	return ":x: Couldn't check project locks, so this request was skipped. Try again shortly."
}
