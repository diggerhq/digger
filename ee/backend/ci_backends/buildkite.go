package ci_backends

import (
	"encoding/json"
	"fmt"
	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"log"
	"strconv"
)

type BuildkiteCi struct {
	Client   buildkite.Client
	Org      string
	Pipeline string
}

func (b BuildkiteCi) TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error {
	log.Printf("Trigger Buildkite Workflow: repoOwner: %v, repoName: %v, commentId: %v", repoOwner, repoName, commentId)
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
	branch := jobSpec.Branch
	commitSha := jobSpec.Commit

	inputs := orchestrator_scheduler.WorkflowInput{
		Id:        job.DiggerJobID,
		JobString: jobString,
		CommentId: strconv.FormatInt(commentId, 10),
		RunName:   fmt.Sprintf("[%v] %v %v By: %v PR: %v", batchIdShort, diggerCommand, projectName, requestedBy, prNumber),
	}

	client := b.Client
	_, _, err = client.Builds.Create(b.Org, b.Pipeline, &buildkite.CreateBuild{
		Commit:  commitSha,
		Branch:  branch,
		Message: inputs.RunName,
	})

	return err

}
