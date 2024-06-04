package ci_backends

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/go-github/v61/github"
	"log"
	"strconv"
)

type GithubActionCi struct {
	Client *github.Client
}

func (g GithubActionCi) TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error {
	log.Printf("TriggerGithubWorkflow: repoOwner: %v, repoName: %v, commentId: %v", repoOwner, repoName, commentId)
	client := g.Client
	var jobSpec orchestrator.JobJson
	err := json.Unmarshal([]byte(jobString), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return fmt.Errorf("could not marshal json string: %v", err)
	}

	batchIdShort := job.Batch.ID.String()[:8]
	diggerCommand := fmt.Sprintf("digger %v", job.Batch.BatchType)
	projectName := jobSpec.ProjectName
	requestedBy := jobSpec.RequestedBy
	prNumber := *jobSpec.PullRequestNumber
	inputs := orchestrator_scheduler.WorkflowInput{
		Id:        job.DiggerJobID,
		JobString: jobString,
		CommentId: strconv.FormatInt(commentId, 10),
		RunName:   fmt.Sprintf("[%v] %v %v By: %v PR: %v", batchIdShort, diggerCommand, projectName, requestedBy, prNumber),
	}

	_, err = client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), repoOwner, repoName, job.WorkflowFile, github.CreateWorkflowDispatchEventRequest{
		Ref:    job.Batch.BranchName,
		Inputs: inputs.ToMap(),
	})

	return err
}
