package services

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/samber/lo"
	"log"
	"os"
	"strconv"
	"strings"
)

func GetVCSTokenFromJob(job models.DiggerJob, gh utils.GithubClientProvider) (*string, error) {
	// TODO: make it VCS generic
	batch := job.Batch
	var token string
	switch batch.VCS {
	case models.DiggerVCSGithub:
		_, ghToken, err := utils.GetGithubService(
			gh,
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

func getVariablesSpecFromEnvMap(envVars map[string]string, stage string) []spec.VariableSpec {
	variablesSpec := make([]spec.VariableSpec, 0)
	for k, v := range envVars {
		if strings.HasPrefix(v, "$DIGGER_") {
			val := strings.ReplaceAll(v, "$DIGGER_", "")
			variablesSpec = append(variablesSpec, spec.VariableSpec{
				Name:           k,
				Value:          val,
				IsSecret:       false,
				IsInterpolated: true,
				Stage:          stage,
			})
		} else {
			variablesSpec = append(variablesSpec, spec.VariableSpec{
				Name:           k,
				Value:          v,
				IsSecret:       false,
				IsInterpolated: false,
				Stage:          stage,
			})

		}
	}
	return variablesSpec
}

func findDuplicatesInStage(variablesSpec []spec.VariableSpec, stage string) (error) {
	// Extract the names from VariableSpec
	justNames := lo.Map(variablesSpec, func(item spec.VariableSpec, i int) string {
		return item.Name
	})

	// Group names by their occurrence
	nameCounts := lo.CountValues(justNames)

	// Filter names that occur more than once
	duplicates := lo.Keys(lo.Filter(nameCounts, func(count int, name string) bool {
		return count > 1
	}))

	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate variable names found in '%s' stage: %v", stage, strings.Join(duplicates, ", "))
	}

	return nil // No duplicates found
}

func GetSpecFromJob(job models.DiggerJob) (*spec.Spec, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		log.Printf("could not unmarshal job string: %v", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	stateVariables := getVariablesSpecFromEnvMap(jobSpec.StateEnvVars, "state")
	commandVariables := getVariablesSpecFromEnvMap(jobSpec.CommandEnvVars, "commands")
	runVariables := getVariablesSpecFromEnvMap(jobSpec.RunEnvVars, "run")

	if err := findDuplicatesInStage(stateVariables, "state"); err != nil {
		return nil, err
	}
	if err := findDuplicatesInStage(commandVariables, "commands"); err != nil {
		return nil, err
	}
	if err := findDuplicatesInStage(runVariables, "run"); err != nil {
		return nil, err
	}

	variablesSpec := make([]spec.VariableSpec, 0)
	variablesSpec = append(variablesSpec, stateVariables...)
	variablesSpec = append(variablesSpec, commandVariables...)
	variablesSpec = append(variablesSpec, runVariables...)

	batch := job.Batch

	spec := spec.Spec{
		JobId:     job.DiggerJobID,
		CommentId: strconv.FormatInt(*batch.CommentId, 10),
		Job:       jobSpec,
		Reporter: spec.ReporterSpec{
			ReportingStrategy:     "comments_per_run",
			ReporterType:          "lazy",
			ReportTerraformOutput: batch.ReportTerraformOutputs,
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
		Variables: variablesSpec,
		Policy: spec.PolicySpec{
			PolicyType: "http",
		},
		CommentUpdater: spec.CommentUpdaterSpec{
			CommentUpdaterType: digger_config.CommentRenderModeBasic,
		},
	}
	return &spec, nil
}
