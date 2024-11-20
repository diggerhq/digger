package comment_updater

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"log"
)

type CommentUpdater interface {
	UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error
}

type BasicCommentUpdater struct {
}

func (b BasicCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	jobSpecs, err := scheduler.GetJobSpecs(jobs)
	if err != nil {
		log.Printf("could not get jobspecs: %v", err)
		return err
	}
	firstJobSpec := jobSpecs[0]
	jobType := firstJobSpec.JobType
	jobTypeTitle := cases.Title(language.AmericanEnglish).String(string(jobType))
	message := ""
	message = message + fmt.Sprintf("| Project | Status | %v | + | ~ | - |\n", jobTypeTitle)
	message = message + fmt.Sprintf("|---------|--------|------|---|---|---|\n")

	for i, job := range jobs {
		jobSpec := jobSpecs[i]
		prCommentUrl := job.PRCommentUrl
		message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>%v</a> | %v | %v | %v|\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), prCommentUrl, jobTypeTitle, job.ResourcesCreated, job.ResourcesUpdated, job.ResourcesDeleted)
	}

	const GithubCommentMaxLength = 65536
	if len(message) > GithubCommentMaxLength {
		// TODO: Handle the case where message is too long by trimming
		log.Printf("WARN: message is too long, trimming")
		log.Printf(message)
		const footer = "[trimmed]"
		trimLength := len(message) - GithubCommentMaxLength + len(footer)
		message = message[:len(message)-trimLength] + footer
	}

	err = prService.EditComment(prNumber, prCommentId, message)
	if err != nil {
		log.Printf("WARNING: failed to update summary comment: %v", err)
	}
	return nil
}

type NoopCommentUpdater struct {
}

func (b NoopCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	return nil
}
