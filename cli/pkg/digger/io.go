package digger

import (
	"fmt"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/goccy/go-json"
	"log"
)

func UpdateStatusComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error {

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

func UpdateAggregateStatus(batch *scheduler.SerializedBatch, prService orchestrator.PullRequestService) error {
	// TODO: Introduce batch-level
	isPlan, err := batch.IsPlan()
	if err != nil {
		log.Printf("failed to get batch job plan/apply status: %v", err)
		return fmt.Errorf("failed to get batch job plan/apply status: %v", err)
	}

	if isPlan {
		prService.SetStatus(batch.PrNumber, batch.ToStatusCheck(), "digger/plan")
		prService.SetStatus(batch.PrNumber, "pending", "digger/apply")
	} else {
		prService.SetStatus(batch.PrNumber, "success", "digger/plan")
		prService.SetStatus(batch.PrNumber, batch.ToStatusCheck(), "digger/apply")
	}
	return nil
}
