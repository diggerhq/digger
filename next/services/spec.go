package services

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"github.com/diggerhq/digger/next/utils"
	"log"
	"os"
	"strconv"
)

func GetVCSTokenFromJob(job model.DiggerJob, gh utils.GithubClientProvider) (*string, error) {
	// TODO: make it VCS generic
	batchId := job.BatchID
	batch, err := dbmodels.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("could not get digger batch: %v", err)
		return nil, fmt.Errorf("could not get digger batch: %v", err)
	}
	var token string
	switch batch.Vcs {
	case string(dbmodels.DiggerVCSGithub):
		_, ghToken, err := utils.GetGithubService(
			gh,
			batch.GithubInstallationID,
			batch.RepoFullName,
			batch.RepoOwner,
			batch.RepoName,
		)
		token = *ghToken
		if err != nil {
			return nil, fmt.Errorf("TriggerWorkflow: could not retrieve token: %v", err)
		}
	case string(dbmodels.DiggerVCSGitlab):
		token = os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
	default:
		return nil, fmt.Errorf("unknown batch VCS: %v", batch.Vcs)
	}

	return &token, nil
}

func GetRunNameFromJob(job model.DiggerJob) (*string, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.JobSpec), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	batchId := job.BatchID
	batch, err := dbmodels.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("could not get digger batch: %v", err)
		return nil, fmt.Errorf("could not get digger batch: %v", err)
	}

	batchIdShort := batch.ID[:8]
	diggerCommand := fmt.Sprintf("digger %v", batch.BatchType)
	projectName := jobSpec.ProjectName
	requestedBy := jobSpec.RequestedBy
	prNumber := *jobSpec.PullRequestNumber

	runName := fmt.Sprintf("[%v] %v %v By: %v PR: %v", batchIdShort, diggerCommand, projectName, requestedBy, prNumber)
	return &runName, nil
}

func GetSpecFromJob(job model.DiggerJob) (*spec.Spec, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.JobSpec), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	batchId := job.BatchID
	batch, err := dbmodels.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("could not get digger batch: %v", err)
		return nil, fmt.Errorf("could not get digger batch: %v", err)
	}

	spec := spec.Spec{
		SpecType:  spec.SpecTypePullRequestJob,
		JobId:     job.DiggerJobID,
		CommentId: strconv.FormatInt(batch.CommentID, 10),
		Job:       jobSpec,
		Reporter: spec.ReporterSpec{
			ReportingStrategy:     "comments_per_run",
			ReporterType:          "lazy",
			ReportTerraformOutput: true,
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
			VcsType:      string(batch.Vcs),
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
