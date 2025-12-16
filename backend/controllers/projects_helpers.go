package controllers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/digger_config"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
)


func GenerateChecksSummaryForBatch( batch *models.DiggerBatch) (string, error) {
	summaryEndpoint := os.Getenv("DIGGER_AI_SUMMARY_ENDPOINT")
	if summaryEndpoint == "" {
		slog.Error("DIGGER_AI_SUMMARY_ENDPOINT not set")
		return"", fmt.Errorf("could not generate AI summary, ai summary endpoint missing")
	}
	apiToken := os.Getenv("DIGGER_AI_SUMMARY_API_TOKEN")

	jobs, err := models.DB.GetDiggerJobsForBatch(batch.ID)
	if err != nil {
		slog.Error("Could not get jobs for batch",
			"batchId", batch.ID,
			"error", err,
		)

		return "", fmt.Errorf("could not get jobs for batch: %v", err)
	}

	terraformOutputs := ""
	for _, job := range jobs {
		var jobSpec orchestrator_scheduler.JobJson
		err := json.Unmarshal(job.SerializedJobSpec, &jobSpec)
		if err != nil {
			slog.Error("Could not unmarshal job spec",
				"jobId", job.DiggerJobID,
				"error", err,
			)

			return "", fmt.Errorf("could not summarize plans due to unmarshalling error: %v", err)
		}

		projectName := jobSpec.ProjectName
		slog.Debug("Adding Terraform output for project",
			"projectName", projectName,
			"jobId", job.DiggerJobID,
			"outputLength", len(job.TerraformOutput),
		)

		terraformOutputs += fmt.Sprintf("<PLAN_START>terraform output for %v: %v <PLAN_END>\n\n", projectName, job.TerraformOutput)
	}

	aiSummary, err := utils.GetAiSummaryFromTerraformPlans(terraformOutputs, summaryEndpoint, apiToken)
	if err != nil {
		slog.Error("Could not generate AI summary from Terraform outputs",
			"batchId", batch.ID,
			"error", err,
		)

		return "", fmt.Errorf("could not summarize terraform outputs: %v", err)
	}

	summary := ""
	if aiSummary != "FOUR_OH_FOUR" {
		summary = fmt.Sprintf(":sparkles: **AI summary (experimental):** %v", aiSummary)
	}

	return summary, nil
}

func GenerateChecksSummaryForJob( job *models.DiggerJob) (string, error) {
	batch := job.Batch
	summaryEndpoint := os.Getenv("DIGGER_AI_SUMMARY_ENDPOINT")
	if summaryEndpoint == "" {
		slog.Error("AI summary endpoint not configured", "batch", batch.ID, "jobId", job.ID, "DiggerJobId", job.DiggerJobID)
		return"", fmt.Errorf("could not generate AI summary, ai summary endpoint missing")
	}
	apiToken := os.Getenv("DIGGER_AI_SUMMARY_API_TOKEN")

	if job.TerraformOutput == "" {
		slog.Warn("Terraform output not set yet, ignoring this call")
		return "", nil
	}
	terraformOutput := fmt.Sprintf("<PLAN_START>Terraform output for: %v<PLAN_END>\n\n", job.TerraformOutput)
	aiSummary, err := utils.GetAiSummaryFromTerraformPlans(terraformOutput, summaryEndpoint, apiToken)
	if err != nil {
		slog.Error("Could not generate AI summary from Terraform outputs",
			"batchId", batch.ID,
			"error", err,
		)

		return "", fmt.Errorf("could not summarize terraform outputs: %v", err)
	}

	summary := ""

	if job.WorkflowRunUrl != nil {
		summary += fmt.Sprintf(":link: <a href='%v'>CI job</a>\n\n", *job.WorkflowRunUrl )
	}

	if aiSummary != "FOUR_OH_FOUR" {
		summary += fmt.Sprintf(":sparkles: **AI summary (experimental):** %v", aiSummary)
	}

	return summary, nil
}


func UpdateCheckRunForBatch(gh utils.GithubClientProvider, batch *models.DiggerBatch) error {
	slog.Info("Updating PR status for batch",
		"batchId", batch.ID,
		"prNumber", batch.PrNumber,
		"batchStatus", batch.Status,
		"batchType", batch.BatchType,
	)

	if batch.CheckRunId == nil {
		slog.Warn("Check Run update skipped - CheckRunId is nil",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"batchStatus", batch.Status,
			"reason", "checkRunId_nil")
		return fmt.Errorf("error checking run id, found nil batch")
	}

	if batch.VCS != models.DiggerVCSGithub {
		slog.Warn("Check Run update skipped - non-GitHub VCS",
			"batchId", batch.ID,
			"vcs", batch.VCS,
			"prNumber", batch.PrNumber,
			"reason", "non_github_vcs")
		return fmt.Errorf("We only support github VCS for modern checks at the moment")
	}
	prService, err := utils.GetPrServiceFromBatch(batch, gh)
	if err != nil {
		slog.Warn("Check Run update failed - could not get PR service",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "pr_service_unavailable")
		return fmt.Errorf("error getting github service: %v", err)
	}

	ghPrService := prService.(*github.GithubService)
	diggerYmlString := batch.DiggerConfig
	diggerConfigYml, err := digger_config.LoadDiggerConfigYamlFromString(diggerYmlString)
	if err != nil {
		slog.Warn("Check Run update failed - could not load Digger config",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "config_load_failed")
		return fmt.Errorf("error loading digger config from batch: %v", err)
	}

	config, _, err := digger_config.ConvertDiggerYamlToConfig(diggerConfigYml)
	if err != nil {
		slog.Warn("Check Run update failed - could not convert Digger YAML to config",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "config_conversion_failed")
		return fmt.Errorf("error converting Digger YAML to config: %v", err)
	}

	disableDiggerApplyStatusCheck := config.DisableDiggerApplyStatusCheck

	isPlanBatch := batch.BatchType == orchestrator_scheduler.DiggerCommandPlan

	serializedBatch, err := batch.MapToJsonStruct()
	if err != nil {
		slog.Warn("Check Run update failed - could not serialize batch",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "batch_serialization_failed")
		return fmt.Errorf("error mapping batch to json struct: %v", err)
	}
	slog.Debug("Updating PR status for batch",
		"batchId", batch.ID, "prNumber", batch.PrNumber, "batchStatus", batch.Status, "batchType", batch.BatchType,
		"newStatus", serializedBatch.ToCheckRunStatus())

	jobs, err := models.DB.GetDiggerJobsForBatch(batch.ID)
	if err != nil {
		slog.Warn("Check Run update failed - could not get jobs for batch",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "jobs_fetch_failed")
		return fmt.Errorf("error getting jobs for batch: %v", err)
	}
	message, err := utils.GenerateRealtimeCommentMessage(jobs, batch.BatchType)
	if err != nil {
		slog.Warn("Check Run update failed - could not generate comment message",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "message_generation_failed")
		return fmt.Errorf("error generating realtime comment message: %v", err)
	}

	var summary = ""
	if batch.Status == orchestrator_scheduler.BatchJobSucceeded || batch.Status == orchestrator_scheduler.BatchJobFailed {
		summary, err = GenerateChecksSummaryForBatch(batch)
		if err != nil {
			slog.Warn("Error generating checks summary for batch", "batchId", batch.ID, "error", err)
		}
	}

	if isPlanBatch {
		status := serializedBatch.ToCheckRunStatus()
		conclusion := serializedBatch.ToCheckRunConclusion()
		title := "Plans Summary"
		opts := github.GithubCheckRunUpdateOptions{
			&status,
			conclusion,
			&title,
			&summary,
			&message,
			utils.GetActionsForBatch(batch),
		}
		_, err = ghPrService.UpdateCheckRun(*batch.CheckRunId, opts)
		if err != nil {
			slog.Warn("GitHub Check Run API call failed for batch plan",
				"operation", "UpdateCheckRun",
				"batchId", batch.ID,
				"checkRunId", *batch.CheckRunId,
				"prNumber", batch.PrNumber,
				"batchStatus", batch.Status,
				"batchType", "plan",
				"error", err,
				"errorType", fmt.Sprintf("%T", err),
				"reason", "github_api_failed")
		} else {
			slog.Debug("Successfully updated Check Run for batch plan",
				"batchId", batch.ID,
				"checkRunId", *batch.CheckRunId,
				"status", status)
		}
	} else {
		if disableDiggerApplyStatusCheck == false {
			status := serializedBatch.ToCheckRunStatus()
			conclusion := serializedBatch.ToCheckRunConclusion()
			title := "Apply Summary"
			opts := github.GithubCheckRunUpdateOptions{
				&status,
				conclusion,
				&title,
				&summary,
				&message,
				utils.GetActionsForBatch(batch),
			}
			_, err = ghPrService.UpdateCheckRun(*batch.CheckRunId, opts)
			if err != nil {
				slog.Warn("GitHub Check Run API call failed for batch apply",
					"operation", "UpdateCheckRun",
					"batchId", batch.ID,
					"checkRunId", *batch.CheckRunId,
					"prNumber", batch.PrNumber,
					"batchStatus", batch.Status,
					"batchType", "apply",
					"error", err,
					"errorType", fmt.Sprintf("%T", err),
					"reason", "github_api_failed")
			} else {
				slog.Debug("Successfully updated Check Run for batch apply",
					"batchId", batch.ID,
					"checkRunId", *batch.CheckRunId,
					"status", status)
			}
		}
	}
	return nil
}


// more modern check runs on github have their own page
func UpdateCheckRunForJob(gh utils.GithubClientProvider, job *models.DiggerJob) error {
	batch := job.Batch
	slog.Info("Updating PR Check run for job",
		"jobId", job.DiggerJobID,
		"prNumber", batch.PrNumber,
		"jobStatus", job.Status,
		"batchType", batch.BatchType,
	)

	if batch.VCS != models.DiggerVCSGithub {
		slog.Warn("Check Run update skipped for job - non-GitHub VCS",
			"jobId", job.DiggerJobID,
			"batchId", batch.ID,
			"vcs", batch.VCS,
			"prNumber", batch.PrNumber,
			"reason", "non_github_vcs")
		return fmt.Errorf("Error updating PR status for job only github is supported")
	}

	if job.CheckRunId == nil {
		slog.Warn("Check Run update skipped for job - CheckRunId is nil",
			"jobId", job.DiggerJobID,
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"jobStatus", job.Status,
			"reason", "checkRunId_nil")
		return fmt.Errorf("Error updating PR status, could not find checkRunId in job")
	}

	prService, err := utils.GetPrServiceFromBatch(batch, gh)
	ghService := prService.(*github.GithubService)

	if err != nil {
		slog.Warn("Check Run update failed for job - could not get PR service",
			"jobId", job.DiggerJobID,
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "pr_service_unavailable")
		return fmt.Errorf("error getting github service: %v", err)
	}

	var jobSpec orchestrator_scheduler.JobJson
	err = json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		slog.Warn("Check Run update failed for job - could not unmarshal job spec",
			"jobId", job.DiggerJobID,
			"batchId", batch.ID,
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"reason", "job_spec_unmarshal_failed")
		return fmt.Errorf("could not unmarshal json string: %v", err)
	}

	isPlan := jobSpec.IsPlan()
	status, err := models.GetCheckRunStatusForJob(job)
	if err != nil {
		slog.Warn("Failed to get check run status for job",
			"jobId", job.DiggerJobID,
			"jobStatus", job.Status,
			"error", err)
		return fmt.Errorf("could not get status check for job: %v", err)
	}

	conclusion, err := models.GetCheckRunConclusionForJob(job)
	if err != nil {
		slog.Warn("Failed to get check run conclusion for job",
			"jobId", job.DiggerJobID,
			"jobStatus", job.Status,
			"error", err)
		return fmt.Errorf("could not get conclusion for job: %v", err)
	}
	
	// Validate status and conclusion before sending to GitHub
	validStatuses := map[string]bool{"queued": true, "in_progress": true, "completed": true}
	validConclusions := map[string]bool{"": true, "success": true, "failure": true, "neutral": true, "cancelled": true, "timed_out": true, "action_required": true, "skipped": true}
	
	if !validStatuses[status] {
		slog.Warn("Invalid Check Run status detected",
			"jobId", job.DiggerJobID,
			"jobStatus", job.Status,
			"checkRunStatus", status,
			"validStatuses", []string{"queued", "in_progress", "completed"})
	}
	
	if !validConclusions[conclusion] {
		slog.Warn("Invalid Check Run conclusion detected",
			"jobId", job.DiggerJobID,
			"jobStatus", job.Status,
			"checkRunConclusion", conclusion,
			"validConclusions", []string{"", "success", "failure", "neutral", "cancelled", "timed_out", "action_required", "skipped"})
	}
	
	slog.Debug("Preparing to update Check Run for job",
		"jobId", job.DiggerJobID,
		"jobStatus", job.Status,
		"checkRunStatus", status,
		"checkRunConclusion", conclusion,
		"isPlan", isPlan)

	// Only pass conclusion to GitHub API when job is completed (non-empty conclusion).
	// GitHub rejects empty string as conclusion - it must be omitted for in-progress jobs.
	var conclusionPtr *string
	if conclusion != "" {
		conclusionPtr = &conclusion
	}

	text := "" +
		"```terraform\n" +
		job.TerraformOutput +
		"```\n"


	var summary = ""
	if job.Status == orchestrator_scheduler.DiggerJobSucceeded || job.Status == orchestrator_scheduler.DiggerJobFailed {
		summary, err = GenerateChecksSummaryForJob(job)
		if err != nil {
			slog.Warn("Error generating checks summary for batch", "batchId", batch.ID, "error", err)
		}
	}


	slog.Debug("Updating PR status for job", "jobId", job.DiggerJobID, "status", status, "conclusion", conclusion)
	if isPlan {
		title := fmt.Sprintf("%v to create %v to update %v to delete", job.DiggerJobSummary.ResourcesCreated, job.DiggerJobSummary.ResourcesUpdated, job.DiggerJobSummary.ResourcesDeleted)
		opts := github.GithubCheckRunUpdateOptions{
			Status:     &status,
			Conclusion: conclusionPtr,
			Title:      &title,
			Summary:    &summary,
			Text:       &text,
			Actions:    utils.GetActionsForJob(job),
		}
		_, err = ghService.UpdateCheckRun(*job.CheckRunId, opts)
		if err != nil {
			slog.Warn("GitHub Check Run API call failed for job plan",
				"operation", "UpdateCheckRun",
				"jobId", job.DiggerJobID,
				"checkRunId", *job.CheckRunId,
				"batchId", batch.ID,
				"prNumber", batch.PrNumber,
				"jobStatus", job.Status,
				"jobType", "plan",
				"error", err,
				"errorType", fmt.Sprintf("%T", err),
				"reason", "github_api_failed")
		} else {
			slog.Debug("Successfully updated Check Run for job plan",
				"jobId", job.DiggerJobID,
				"checkRunId", *job.CheckRunId,
				"status", status)
		}
	} else {
		title := fmt.Sprintf("%v created %v updated %v deleted", job.DiggerJobSummary.ResourcesCreated, job.DiggerJobSummary.ResourcesUpdated, job.DiggerJobSummary.ResourcesDeleted)
		opts := github.GithubCheckRunUpdateOptions{
			Status:     &status,
			Conclusion: conclusionPtr,
			Title:      &title,
			Summary:    &summary,
			Text:       &text,
			Actions:    utils.GetActionsForJob(job),
		}
		_, err = ghService.UpdateCheckRun(*job.CheckRunId, opts)
		if err != nil {
			slog.Warn("GitHub Check Run API call failed for job apply",
				"operation", "UpdateCheckRun",
				"jobId", job.DiggerJobID,
				"checkRunId", *job.CheckRunId,
				"batchId", batch.ID,
				"prNumber", batch.PrNumber,
				"jobStatus", job.Status,
				"jobType", "apply",
				"error", err,
				"errorType", fmt.Sprintf("%T", err),
				"reason", "github_api_failed")
		} else {
			slog.Debug("Successfully updated Check Run for job apply",
				"jobId", job.DiggerJobID,
				"checkRunId", *job.CheckRunId,
				"status", status)
		}
	}
	return nil
}
