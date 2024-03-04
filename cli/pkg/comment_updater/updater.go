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

	message := ":construction_worker: Jobs status:\n\n"
	for _, job := range jobs {
		var jobSpec orchestrator.JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job, %v", err)
			return fmt.Errorf("Failed to unmarshall serialized job: %v", err)
		}
		isPlan := jobSpec.IsPlan()

		message = message + fmt.Sprintf("<!-- PROJECTHOLDER %v -->\n", job.ProjectName)
		message = message + fmt.Sprintf("%v **%v** <a href='%v'>%v</a>%v\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), job.ResourcesSummaryString(isPlan))
		message = message + fmt.Sprintf("<!-- PROJECTHOLDEREND %v -->\n", job.ProjectName)
	}

	prService.EditComment(prNumber, prCommentId, message)
	return nil
}

type NoopCommentUpdater struct {
}

func (b NoopCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error {
	return nil
}
