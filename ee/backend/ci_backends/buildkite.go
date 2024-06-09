package ci_backends

import (
	"encoding/json"
	"fmt"
	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/spec"
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

	runName := fmt.Sprintf("[%v] %v %v By: %v PR: %v", batchIdShort, diggerCommand, projectName, requestedBy, prNumber)
	spec := spec.Spec{
		JobId:     job.DiggerJobID,
		CommentId: strconv.FormatInt(commentId, 10),
		RunName:   runName,
		Job:       jobSpec,
		Reporter: spec.ReporterSpec{
			ReportingStrategy: "comments_per_run",
			ReporterType:      "lazy",
		},
		Lock: spec.LockSpec{
			LockType: "noop",
		},
		Backend: spec.BackendSpec{
			BackendHostname:         jobSpec.BackendHostname,
			BackendOrganisationName: jobSpec.BackendOrganisationName,
			BackendJobToken:         jobSpec.BackendJobToken,
			BackendType:             "backend",
		},
		VCS: spec.VcsSpec{
			VcsType:   "github",
			Actor:     jobSpec.RequestedBy,
			RepoOwner: repoOwner,
			RepoName:  repoName,
		},
		Policy: spec.PolicySpec{
			PolicyType: "http",
		},
	}

	_, ghToken, err := utils.GetGithubService(
		utils.DiggerGithubRealClientProvider{},
		job.Batch.GithubInstallationId,
		job.Batch.RepoFullName,
		job.Batch.RepoOwner,
		job.Batch.RepoName,
	)
	if err != nil {
		return fmt.Errorf("TriggerWorkflow: could not retrieve token: %v", err)
	}

	specBytes, err := json.Marshal(spec)
	client := b.Client
	_, _, err = client.Builds.Create(b.Org, b.Pipeline, &buildkite.CreateBuild{
		Commit:  commitSha,
		Branch:  branch,
		Message: runName,
		Author:  buildkite.Author{Username: requestedBy},
		Env: map[string]string{
			"DIGGER_SPEC":  string(specBytes),
			"GITHUB_TOKEN": *ghToken,
		},
		PullRequestID: int64(prNumber),
	})

	return err

}
