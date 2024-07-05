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
	isPlan := jobType == string(scheduler.DiggerCommandPlan)
	jobTypeTitle := cases.Title(language.AmericanEnglish).String(string(jobType))
	message := ""
	if isPlan {
		message = message + fmt.Sprintf("| Project | Status | %v | + | ~ | - |\n", jobTypeTitle)
		message = message + fmt.Sprintf("|---------|--------|------|---|---|---|\n")
	} else {
		message = message + fmt.Sprintf("| Project | Status | %v |\n", jobTypeTitle)
		message = message + fmt.Sprintf("|---------|--------|-------|\n")
	}
	for i, job := range jobs {
		jobSpec := jobSpecs[i]
		prCommentUrl := job.PRCommentUrl
		if isPlan {
			message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>%v</a> | %v | %v | %v|\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), prCommentUrl, jobTypeTitle, job.ResourcesCreated, job.ResourcesUpdated, job.ResourcesDeleted)
		} else {
			message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>%v</a> |\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), prCommentUrl, jobTypeTitle)
		}
	}

	prService.EditComment(prNumber, prCommentId, message)
	return nil
}

type NoopCommentUpdater struct {
}

func (b NoopCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	return nil
}
