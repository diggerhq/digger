package controllers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
)

const (
	// How long a plan or apply request waits for another one on the same PR
	// to create its batch.
	commandLockTimeout = time.Minute
	// Longest a request holds the lock, in case a GitHub call hangs.
	commandLockMaxHold = 5 * time.Minute
)

// guardDuplicateCommand stops a plan or apply request that repeats one
// already running on this commit. A plan is skipped when every requested
// project is running; an apply when any is, since two applies of a project
// race for its state. force skips the check. msg says why, or is "" when
// the command should run.
// Other plan and apply requests on this PR wait until release is called, so
// call it once the batch exists; extra calls are no-ops.
func guardDuplicateCommand(command scheduler.DiggerCommand, force bool, repoFullName string, prNumber int, commitSha string, projects []digger_config.Project) (msg string, release func()) {
	release = func() {}
	if command != scheduler.DiggerCommandPlan && command != scheduler.DiggerCommandApply {
		return "", release
	}
	window := config.DuplicateCommandWindow()
	if window == 0 || len(projects) == 0 {
		return "", release
	}
	if force {
		return "", release
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandLockTimeout)
	defer cancel()
	unlock, err := models.DB.LockPullRequestCommands(ctx, repoFullName, prNumber, commandLockMaxHold)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("timed out waiting for the PR command lock, skipping", "command", command, "prNumber", prNumber)
		return fmt.Sprintf(":hourglass: Another `digger plan` or `digger apply` on this PR is still starting, so this one was skipped. Try again shortly, or add `--force` to a `digger %s` comment to run it anyway.", command), release
	}
	cantCheck := fmt.Sprintf(":x: Couldn't check whether `digger %s` is already running, so this one was skipped. Try again, or add `--force` to a `digger %s` comment to run it anyway.", command, command)
	if err != nil {
		slog.Error("could not take the PR command lock, skipping", "command", command, "prNumber", prNumber, "error", err)
		return cantCheck, release
	}
	release = unlock

	running, err := models.DB.GetRunningProjectsForCommit(models.DiggerVCSGithub, repoFullName, prNumber, commitSha, command, time.Now().Add(-window))
	if err != nil {
		slog.Error("could not check for a duplicate command, skipping", "command", command, "prNumber", prNumber, "error", err)
		return cantCheck, release
	}
	var busy []string
	for _, p := range projects {
		if slices.Contains(running, p.Name) {
			// Names come from the PR's digger.yml; keep them inside the code span.
			busy = append(busy, "`"+strings.NewReplacer("`", "", "\r", "", "\n", " ").Replace(p.Name)+"`")
		}
	}
	skip := len(busy) > 0
	if command == scheduler.DiggerCommandPlan {
		skip = len(busy) == len(projects)
	}
	if !skip {
		return "", release
	}
	slog.Info("skipping duplicate command", "command", command, "prNumber", prNumber, "commitSha", commitSha, "projects", busy)
	return fmt.Sprintf(":information_source: `digger %s` is already running on %s for %s, so this one was skipped. Add `--force` to a `digger %s` comment to run it anyway.",
		command, commitSha, strings.Join(busy, ", "), command), release
}
