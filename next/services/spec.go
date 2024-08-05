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

func RefreshVariableSpecForJob(job model.DiggerJob) error {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.JobSpec), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return fmt.Errorf("could not marshal json string: %v", err)
	}

	batchId := job.BatchID
	batch, err := dbmodels.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("could not get digger batch: %v", err)
		return fmt.Errorf("could not get digger batch: %v", err)
	}

	org, err := dbmodels.DB.GetOrganisationById(batch.OrganizationID)
	if err != nil {
		log.Printf("could not get org: %v", err)
		return fmt.Errorf("could not get orb: %v", err)
	}

	repo, err := dbmodels.DB.GetRepo(batch.OrganizationID, batch.RepoName)
	if err != nil {
		log.Printf("could not get repo: %v", err)
		return fmt.Errorf("could not get repo: %v", err)
	}

	project, err := dbmodels.DB.GetProjectByName(org.ID, repo, jobSpec.ProjectName)
	if err != nil {
		log.Printf("could not get repo: %v", err)
		return fmt.Errorf("could not get repo: %v", err)
	}

	variables, err := dbmodels.DB.GetProjectVariables(project.ID)
	if err != nil {
		log.Printf("could not get variables: %v", err)
		return fmt.Errorf("could not get variables: %v", err)
	}

	specVariables := make([]spec.VariableSpec, 0)
	for _, v := range variables {
		specVariables = append(specVariables, dbmodels.ToVariableSpec(v))
	}

	// set some default variables as well
	specVariables = append(specVariables, []spec.VariableSpec{
		{
			Name:     "DIGGER_PROJECT_NAME",
			Value:    project.Name,
			IsSecret: false,
		},
		{
			Name:     "DIGGER_PROJECT_DIR",
			Value:    project.TerraformWorkingDir,
			IsSecret: false,
		},
	}...)

	marshalledSpecVariables, err := json.Marshal(specVariables)
	if err != nil {
		log.Printf("error while marshalling spec variables: %v", err)
		return fmt.Errorf("error while marshalling spec variables: %v", err)
	}

	job.VariablesSpec = marshalledSpecVariables
	dbmodels.DB.GormDB.Save(job)

	// here we get vars using project.ID for this project and load them into a specVariables, marshall
	// them and then store them in job.VarsSpec

	return nil
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

func GetSpecFromJob(job model.DiggerJob, specType spec.SpecType) (*spec.Spec, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal(job.JobSpec, &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	var variablesSpec []spec.VariableSpec
	err = json.Unmarshal(job.VariablesSpec, &variablesSpec)
	if err != nil {
		log.Printf("could not unmarshal variables spec: %v", err)
		return nil, fmt.Errorf("could not unmarshal variables spec: %v", err)
	}

	batchId := job.BatchID
	batch, err := dbmodels.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("could not get digger batch: %v", err)
		return nil, fmt.Errorf("could not get digger batch: %v", err)
	}

	spec := spec.Spec{
		SpecType:  specType,
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
		Variables: variablesSpec,
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
