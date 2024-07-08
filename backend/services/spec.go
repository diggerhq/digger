package services

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"log"
	"os"
	"strconv"
)

func GetVCSTokenFromJob(job models.DiggerJob) (*string, error) {
	// TODO: make it VCS generic
	batch := job.Batch
	var token string
	switch batch.VCS {
	case models.DiggerVCSGithub:
		_, ghToken, err := utils.GetGithubService(
			utils.DiggerGithubRealClientProvider{},
			job.Batch.GithubInstallationId,
			job.Batch.RepoFullName,
			job.Batch.RepoOwner,
			job.Batch.RepoName,
		)
		token = *ghToken
		if err != nil {
			return nil, fmt.Errorf("TriggerWorkflow: could not retrieve token: %v", err)
		}
	case models.DiggerVCSGitlab:
		token = os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
	default:
		return nil, fmt.Errorf("unknown batch VCS: %v", batch.VCS)
	}

	return &token, nil
}

func GetRunNameFromJob(job models.DiggerJob) (*string, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	batch := job.Batch
	batchIdShort := batch.ID.String()[:8]
	diggerCommand := fmt.Sprintf("digger %v", batch.BatchType)
	projectName := jobSpec.ProjectName
	requestedBy := jobSpec.RequestedBy
	prNumber := *jobSpec.PullRequestNumber

	runName := fmt.Sprintf("[%v] %v %v By: %v PR: %v", batchIdShort, diggerCommand, projectName, requestedBy, prNumber)
	return &runName, nil
}

func GetSpecFromJob(job models.DiggerJob) (*spec.Spec, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	batch := job.Batch

	spec := spec.Spec{
		JobId:     job.DiggerJobID,
		CommentId: strconv.FormatInt(*batch.CommentId, 10),
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
			VcsType:      string(batch.VCS),
			Actor:        jobSpec.RequestedBy,
			RepoFullname: batch.RepoFullName,
			RepoOwner:    batch.RepoOwner,
			RepoName:     batch.RepoName,
			WorkflowFile: job.WorkflowFile,
		},
		Policy: spec.PolicySpec{
			PolicyType: "http",
		},
	}
	return &spec, nil
}
