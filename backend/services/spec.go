package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/samber/lo"
)

func GetVCSTokenFromJob(job models.DiggerJob, gh utils.GithubClientProvider) (*string, error) {
	// TODO: make it VCS generic
	batch := job.Batch
	var token string

	slog.Debug("Retrieving VCS token",
		"jobId", job.DiggerJobID,
		slog.Group("vcs",
			slog.String("type", string(batch.VCS)),
			slog.String("repo", batch.RepoFullName),
		),
	)

	switch batch.VCS {
	case models.DiggerVCSGithub:
		_, ghToken, err := utils.GetGithubService(
			gh,
			job.Batch.GithubInstallationId,
			job.Batch.RepoFullName,
			job.Batch.RepoOwner,
			job.Batch.RepoName,
		)
		if err != nil {
			slog.Error("Failed to retrieve GitHub token",
				"jobId", job.DiggerJobID,
				"installationId", job.Batch.GithubInstallationId,
				"error", err)
			return nil, fmt.Errorf("TriggerWorkflow: could not retrieve token: %v", err)
		}
		token = *ghToken

	case models.DiggerVCSGitlab:
		token = os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
		slog.Debug("Using GitLab access token from environment", "jobId", job.DiggerJobID)

	case models.DiggerVCSBitbucket:
		// TODO: Refactor this piece into its own
		if batch.VCSConnectionId == nil {
			slog.Error("Connection ID not set", "jobId", job.DiggerJobID)
			return nil, fmt.Errorf("connection ID not set, could not get vcs token")
		}

		slog.Debug("Using Bitbucket connection", "jobId", job.DiggerJobID, "connectionId", *batch.VCSConnectionId)
		connectionId := strconv.Itoa(int(*batch.VCSConnectionId))
		connectionEncrypted, err := models.DB.GetVCSConnectionById(connectionId)
		if err != nil {
			slog.Error("Failed to fetch connection", "connectionId", connectionId, "error", err)
			return nil, fmt.Errorf("failed to fetch connection: %v", err)
		}

		secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
		if secret == "" {
			slog.Error("No encryption secret specified", "jobId", job.DiggerJobID)
			return nil, fmt.Errorf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
		}

		connectionDecrypted, err := utils.DecryptConnection(connectionEncrypted, []byte(secret))
		if err != nil {
			slog.Error("Could not decrypt connection", "connectionId", connectionId, "error", err)
			return nil, fmt.Errorf("ERROR: could not perform decryption: %v", err)
		}

		token = connectionDecrypted.BitbucketAccessToken

	default:
		slog.Error("Unknown VCS type", "vcsType", batch.VCS, "jobId", job.DiggerJobID)
		return nil, fmt.Errorf("unknown batch VCS: %v", batch.VCS)
	}

	slog.Debug("Successfully retrieved VCS token", "jobId", job.DiggerJobID)
	return &token, nil
}

func GetRunNameFromJob(job models.DiggerJob) (*string, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		slog.Error("Could not unmarshal job spec", "jobId", job.DiggerJobID, "error", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	batch := job.Batch
	batchIdShort := batch.ID.String()[:8]
	diggerCommand := fmt.Sprintf("digger %v", batch.BatchType)
	// Use alias for display, keep original name for logging
	projectName := jobSpec.ProjectName
	projectDisplayName := jobSpec.ProjectName
	if jobSpec.ProjectAlias != "" {
		projectDisplayName = jobSpec.ProjectAlias
	}
	requestedBy := jobSpec.RequestedBy
	prNumber := *jobSpec.PullRequestNumber

	runName := fmt.Sprintf("[%v] %v %v By: %v PR: %v", batchIdShort, diggerCommand, projectDisplayName, requestedBy, prNumber)
	slog.Debug("Generated run name",
		"jobId", job.DiggerJobID,
		"runName", runName,
		slog.Group("components",
			slog.String("batchId", batchIdShort),
			slog.String("command", diggerCommand),
			slog.String("project", projectName),
			slog.String("requestedBy", requestedBy),
			slog.Int("prNumber", prNumber),
		),
	)
	return &runName, nil
}

func getVariablesSpecFromEnvMap(envVars map[string]string) []spec.VariableSpec {
	variablesSpec := make([]spec.VariableSpec, 0)
	for k, v := range envVars {
		if strings.HasPrefix(v, "$DIGGER_") {
			val := strings.ReplaceAll(v, "$DIGGER_", "")
			variablesSpec = append(variablesSpec, spec.VariableSpec{
				Name:           k,
				Value:          val,
				IsSecret:       false,
				IsInterpolated: true,
			})
		} else {
			variablesSpec = append(variablesSpec, spec.VariableSpec{
				Name:           k,
				Value:          v,
				IsSecret:       false,
				IsInterpolated: false,
			})
		}
	}
	return variablesSpec
}

func GetSpecFromJob(job models.DiggerJob) (*spec.Spec, error) {
	var jobSpec scheduler.JobJson
	err := json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		slog.Error("Could not unmarshal job spec", "jobId", job.DiggerJobID, "error", err)
		return nil, fmt.Errorf("could not marshal json string: %v", err)
	}

	variablesSpec := make([]spec.VariableSpec, 0)
	stateVariables := getVariablesSpecFromEnvMap(jobSpec.StateEnvVars)
	commandVariables := getVariablesSpecFromEnvMap(jobSpec.CommandEnvVars)
	runVariables := getVariablesSpecFromEnvMap(jobSpec.RunEnvVars)
	variablesSpec = append(variablesSpec, stateVariables...)
	variablesSpec = append(variablesSpec, commandVariables...)
	variablesSpec = append(variablesSpec, runVariables...)

	// check for duplicates in list of variablesSpec
	justNames := lo.Map(variablesSpec, func(item spec.VariableSpec, i int) string {
		return item.Name
	})
	hasDuplicates := len(justNames) != len(lo.Uniq(justNames))
	if hasDuplicates {
		slog.Error("Duplicate variable names found", "jobId", job.DiggerJobID)
		return nil, fmt.Errorf("could not load variables due to duplicates")
	}

	batch := job.Batch

	var commentId string
	if batch.CommentId != nil {
		commentId = strconv.FormatInt(*batch.CommentId, 10)
	} else {
		commentId = ""
		slog.Warn("Comment ID is nil", "jobId", job.DiggerJobID)
	}

	spec := spec.Spec{
		JobId:     job.DiggerJobID,
		CommentId: commentId,
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

	slog.Debug("Successfully created spec",
		"jobId", job.DiggerJobID,
		slog.Group("spec",
			slog.String("vcsType", string(batch.VCS)),
			slog.String("repo", batch.RepoFullName),
			slog.Int("variableCount", len(variablesSpec)),
		),
	)

	return &spec, nil
}
