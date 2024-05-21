package comment_updater

import (
	"fmt"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"log"
)

type CommentUpdater interface {
	UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error
}

type BasicCommentUpdater struct {
}

func (b BasicCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error {
	jobSpecs, err := scheduler.GetJobSpecs(jobs)
	if err != nil {
		log.Printf("could not get jobspecs: %v", err)
		return err
	}
	firstJobSpec := jobSpecs[0]
	isPlan := firstJobSpec.IsPlan()

	message := ""
	if isPlan {
		message = message + fmt.Sprintf("| Project | Status | Plan | + | ~ | - |\n")
		message = message + fmt.Sprintf("|---------|--------|------|---|---|---|\n")
	} else {
		message = message + fmt.Sprintf("| Project | Status | Apply |\n")
		message = message + fmt.Sprintf("|---------|--------|-------|\n")
	}
	for i, job := range jobs {
		jobSpec := jobSpecs[i]
		prCommentUrl := job.PRCommentUrl
		if isPlan {
			message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>plan</a> | %v | %v | %v|\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), prCommentUrl, job.ResourcesCreated, job.ResourcesUpdated, job.ResourcesDeleted)
		} else {
			message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>apply</a> |\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), prCommentUrl)
		}
	}

	prService.EditComment(prNumber, prCommentId, message)
	return nil
}

type NoopCommentUpdater struct {
}

func (b NoopCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error {
	return nil
}
