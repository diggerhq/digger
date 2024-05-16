package comment_updater

import (
	"encoding/json"
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
		message = message + fmt.Sprintf("| Project | Status | + | ~ | - |\n")
		message = message + fmt.Sprintf("|---------|--------|---|---|---|\n")
	} else {
		message = message + fmt.Sprintf("| Project | Status |\n")
		message = message + fmt.Sprintf("|---------|--------|\n")
	}
	for _, job := range jobs {
		var jobSpec orchestrator.JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job, %v", err)
			return fmt.Errorf("Failed to unmarshall serialized job: %v", err)
		}

		if isPlan {
			message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | %v | %v | %v|\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), job.ResourcesCreated, job.ResourcesUpdated, job.ResourcesDeleted)
		} else {
			message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> |\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString())
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
