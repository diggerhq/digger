package ci_backends

import (
	"context"
	"github.com/diggerhq/digger/backend/models"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/go-github/v61/github"
	"log"
	"strconv"
)

type GithubActionCi struct {
	Client *github.Client
}

func (g GithubActionCi) TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error {
	client := g.Client
	log.Printf("TriggerGithubWorkflow: repoOwner: %v, repoName: %v, commentId: %v", repoOwner, repoName, commentId)
	inputs := orchestrator_scheduler.WorkflowInput{
		Id:        job.DiggerJobID,
		JobString: jobString,
		CommentId: strconv.FormatInt(commentId, 10),
	}
	_, err := client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), repoOwner, repoName, "digger_workflow.yml", github.CreateWorkflowDispatchEventRequest{
		Ref:    job.Batch.BranchName,
		Inputs: inputs.ToMap(),
	})

	return err

}
