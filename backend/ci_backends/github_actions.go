package ci_backends

import (
	"context"
	"github.com/diggerhq/digger/backend/models"
	"github.com/google/go-github/v58/github"
	"log"
	"strconv"
)

type GithubActionCi struct {
	Client *github.Client
}

func (g GithubActionCi) TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error {
	client := g.Client
	log.Printf("TriggerGithubWorkflow: repoOwner: %v, repoName: %v, commentId: %v", repoOwner, repoName, commentId)
	_, err := client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), repoOwner, repoName, "digger_workflow.yml", github.CreateWorkflowDispatchEventRequest{
		Ref:    job.Batch.BranchName,
		Inputs: map[string]interface{}{"job": jobString, "id": job.DiggerJobID, "comment_id": strconv.FormatInt(commentId, 10)},
	})

	return err

}
