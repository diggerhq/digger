package comment_updater

import (
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type CommentUpdater interface {
	UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error
}

type BasicCommentUpdater struct{}

func (b BasicCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	jobSpecs, err := scheduler.GetJobSpecs(jobs)
	if err != nil {
		slog.Error("could not get jobspecs", "error", err, "jobCount", len(jobs))
		return err
	}

	firstJobSpec := jobSpecs[0]
	jobType := firstJobSpec.JobType
	jobTypeTitle := cases.Title(language.AmericanEnglish).String(jobType)

	slog.Info("updating comment with job results",
		"prNumber", prNumber,
		"commentId", prCommentId,
		"jobCount", len(jobs),
		"jobType", jobType)

	message := ""
	message += fmt.Sprintf("| Project | Status | %v | + | ~ | - |\n", jobTypeTitle)
	message += "|---------|--------|------|---|---|---|\n"

	for i, job := range jobs {
		jobSpec := jobSpecs[i]
		prCommentUrl := job.PRCommentUrl

		// Safe handling of WorkflowRunUrl pointer
		workflowUrl := "#"
		if job.WorkflowRunUrl != nil {
			workflowUrl = *job.WorkflowRunUrl
		}

		message += fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>%v</a> | %v | %v | %v|\n",
			job.Status.ToEmoji(),
			jobSpec.ProjectName,
			workflowUrl,
			job.Status.ToString(),
			prCommentUrl,
			jobTypeTitle,
			job.ResourcesCreated,
			job.ResourcesUpdated,
			job.ResourcesDeleted)
	}

	const GithubCommentMaxLength = 65536
	if len(message) > GithubCommentMaxLength {
		// TODO: Handle the case where message is too long by trimming
		slog.Warn("message is too long, trimming",
			"originalLength", len(message),
			"maxLength", GithubCommentMaxLength)

		const footer = "[trimmed]"
		trimLength := len(message) - GithubCommentMaxLength + len(footer)
		message = message[:len(message)-trimLength] + footer

		slog.Debug("trimmed message", "newLength", len(message))
	}

	err = prService.EditComment(prNumber, prCommentId, message)
	if err != nil {
		slog.Warn("failed to update summary comment",
			"error", err,
			"prNumber", prNumber,
			"commentId", prCommentId)
	} else {
		slog.Info("successfully updated summary comment",
			"prNumber", prNumber,
			"commentId", prCommentId)
	}

	return nil
}

type NoopCommentUpdater struct{}

func (b NoopCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	slog.Debug("noop comment updater called, no action taken",
		"prNumber", prNumber,
		"commentId", prCommentId,
		"jobCount", len(jobs))
	return nil
}
