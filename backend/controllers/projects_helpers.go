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
		summary = fmt.Sprintf(":sparkles: **AI summary:** %v", aiSummary)
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
		summary += fmt.Sprintf(":sparkles: **AI summary:** %v", aiSummary)
	}

	return summary, nil
}




func UpdateCommitStatusForBatch(gh utils.GithubClientProvider, batch *models.DiggerBatch) error {
	slog.Info("Updating PR status for batch",
		"batchId", batch.ID,
		"prNumber", batch.PrNumber,
		"batchStatus", batch.Status,
		"batchType", batch.BatchType,
	)

	prService, err := utils.GetPrServiceFromBatch(batch, gh)
	if err != nil {
		slog.Error("Error getting PR service",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error getting github service: %v", err)
	}

	diggerYmlString := batch.DiggerConfig
	diggerConfigYml, err := digger_config.LoadDiggerConfigYamlFromString(diggerYmlString)
	if err != nil {
		slog.Error("Error loading Digger config from batch",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error loading digger config from batch: %v", err)
	}

	config, _, err := digger_config.ConvertDiggerYamlToConfig(diggerConfigYml)
	if err != nil {
		slog.Error("Error converting Digger YAML to config",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error converting Digger YAML to config: %v", err)
	}

	disableDiggerApplyStatusCheck := config.DisableDiggerApplyStatusCheck

	isPlanBatch := batch.BatchType == orchestrator_scheduler.DiggerCommandPlan

	serializedBatch, err := batch.MapToJsonStruct()
	if err != nil {
		slog.Error("Error mapping batch to json struct",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error mapping batch to json struct: %v", err)
	}
	slog.Debug("Updating PR status for batch",
		"batchId", batch.ID, "prNumber", batch.PrNumber, "batchStatus", batch.Status, "batchType", batch.BatchType,
		"newStatus", serializedBatch.ToCommitStatusCheck())
	if isPlanBatch {
		prService.SetStatus(batch.PrNumber, serializedBatch.ToCommitStatusCheck(), "digger/plan")
		if disableDiggerApplyStatusCheck == false {
			prService.SetStatus(batch.PrNumber, "pending", "digger/apply")
		}

	} else {
		prService.SetStatus(batch.PrNumber, "success", "digger/plan")
		if disableDiggerApplyStatusCheck == false {
			prService.SetStatus(batch.PrNumber, serializedBatch.ToCommitStatusCheck(), "digger/apply")
		}
	}
	return nil
}

func UpdateCheckRunForBatch(gh utils.GithubClientProvider, batch *models.DiggerBatch) error {
	slog.Info("Updating PR status for batch",
		"batchId", batch.ID,
		"prNumber", batch.PrNumber,
		"batchStatus", batch.Status,
		"batchType", batch.BatchType,
	)

	if batch.CheckRunId == nil {
		slog.Error("Error checking run id, found nil", "batchId", batch.ID)
		return fmt.Errorf("error checking run id, found nil batch")
	}

	if batch.VCS != models.DiggerVCSGithub {
		return fmt.Errorf("We only support github VCS for modern checks at the moment")
	}
	prService, err := utils.GetPrServiceFromBatch(batch, gh)
	if err != nil {
		slog.Error("Error getting PR service",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error getting github service: %v", err)
	}

	ghPrService := prService.(*github.GithubService)
	diggerYmlString := batch.DiggerConfig
	diggerConfigYml, err := digger_config.LoadDiggerConfigYamlFromString(diggerYmlString)
	if err != nil {
		slog.Error("Error loading Digger config from batch",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error loading digger config from batch: %v", err)
	}

	config, _, err := digger_config.ConvertDiggerYamlToConfig(diggerConfigYml)
	if err != nil {
		slog.Error("Error converting Digger YAML to config",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error converting Digger YAML to config: %v", err)
	}

	disableDiggerApplyStatusCheck := config.DisableDiggerApplyStatusCheck

	isPlanBatch := batch.BatchType == orchestrator_scheduler.DiggerCommandPlan

	serializedBatch, err := batch.MapToJsonStruct()
	if err != nil {
		slog.Error("Error mapping batch to json struct",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error mapping batch to json struct: %v", err)
	}
	slog.Debug("Updating PR status for batch",
		"batchId", batch.ID, "prNumber", batch.PrNumber, "batchStatus", batch.Status, "batchType", batch.BatchType,
		"newStatus", serializedBatch.ToCheckRunStatus())

	jobs, err := models.DB.GetDiggerJobsForBatch(batch.ID)
	if err != nil {
		slog.Error("Error getting jobs for batch",
			"batchId", batch.ID,
			"error", err)
		return fmt.Errorf("error getting jobs for batch: %v", err)
	}
	message, err := utils.GenerateRealtimeCommentMessage(jobs, batch.BatchType)
	if err != nil {
		slog.Error("Error generating realtime comment message",
			"batchId", batch.ID,
			"error", err)
		return fmt.Errorf("error generating realtime comment message: %v", err)
	}

	summary, err := GenerateChecksSummaryForBatch(batch)
	if err != nil {
		slog.Warn("Error generating checks summary for batch", "batchId", batch.ID, "error", err)
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
		ghPrService.UpdateCheckRun(*batch.CheckRunId, opts)
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
			ghPrService.UpdateCheckRun(*batch.CheckRunId, opts)
		}
	}
	return nil
}

func UpdateCommitStatusForJob(gh utils.GithubClientProvider, job *models.DiggerJob) error {
	batch := job.Batch
	slog.Info("Updating PR status for job",
		"jobId", job.DiggerJobID,
		"prNumber", batch.PrNumber,
		"jobStatus", job.Status,
		"batchType", batch.BatchType,
	)

	prService, err := utils.GetPrServiceFromBatch(batch, gh)
	if err != nil {
		slog.Error("Error getting PR service",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error getting github service: %v", err)
	}

	var jobSpec orchestrator_scheduler.JobJson
	err = json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		slog.Error("Could not unmarshal job spec", "jobId", job.DiggerJobID, "error", err)
		return fmt.Errorf("could not unmarshal json string: %v", err)
	}

	isPlan := jobSpec.IsPlan()
	status, err := models.GetCommitStatusForJob(job)
	if err != nil {
		return fmt.Errorf("could not get status check for job: %v", err)
	}
	slog.Debug("Updating PR status for job", "jobId", job.DiggerJobID, "status", status)
	if isPlan {
		prService.SetStatus(batch.PrNumber, status, jobSpec.GetProjectAlias()+"/plan")
		prService.SetStatus(batch.PrNumber, "neutral", jobSpec.GetProjectAlias()+"/apply")
	} else {
		prService.SetStatus(batch.PrNumber, status, jobSpec.GetProjectAlias()+"/apply")
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
		slog.Error("Error updating PR status for job only github is supported", "batchid", batch.ID, "vcs", batch.VCS)
		return fmt.Errorf("Error updating PR status for job only github is supported")
	}

	if job.CheckRunId == nil {
		slog.Error("Error updating PR status, could not find checkRunId in job", "diggerJobId", job.DiggerJobID)
		return fmt.Errorf("Error updating PR status, could not find checkRunId in job")
	}

	prService, err := utils.GetPrServiceFromBatch(batch, gh)
	ghService := prService.(*github.GithubService)

	if err != nil {
		slog.Error("Error getting PR service",
			"batchId", batch.ID,
			"error", err,
		)
		return fmt.Errorf("error getting github service: %v", err)
	}

	var jobSpec orchestrator_scheduler.JobJson
	err = json.Unmarshal([]byte(job.SerializedJobSpec), &jobSpec)
	if err != nil {
		slog.Error("Could not unmarshal job spec", "jobId", job.DiggerJobID, "error", err)
		return fmt.Errorf("could not unmarshal json string: %v", err)
	}

	isPlan := jobSpec.IsPlan()
	status, err := models.GetCheckRunStatusForJob(job)
	if err != nil {
		return fmt.Errorf("could not get status check for job: %v", err)
	}

	conclusion, err := models.GetCheckRunConclusionForJob(job)
	if err != nil {
		return fmt.Errorf("could not get conclusion for job: %v", err)
	}

	text := "" +
		"```terraform\n" +
		job.TerraformOutput +
		"```\n"


	summary, err := GenerateChecksSummaryForJob(job)
	if err != nil {
		slog.Warn("Error generating checks summary for batch", "batchId", batch.ID, "error", err)
	}

	slog.Debug("Updating PR status for job", "jobId", job.DiggerJobID, "status", status, "conclusion", conclusion)
	if isPlan {
		title := fmt.Sprintf("%v to create %v to update %v to delete", job.DiggerJobSummary.ResourcesCreated, job.DiggerJobSummary.ResourcesUpdated, job.DiggerJobSummary.ResourcesDeleted)
		opts := github.GithubCheckRunUpdateOptions{
			Status:     &status,
			Conclusion: &conclusion,
			Title:      &title,
			Summary:    &summary,
			Text:       &text,
			Actions:    utils.GetActionsForJob(job),
		}
		_, err = ghService.UpdateCheckRun(*job.CheckRunId, opts)
		if err != nil {
			slog.Error("Error updating PR status for job", "error", err)
		}
	} else {
		title := fmt.Sprintf("%v created %v updated %v deleted", job.DiggerJobSummary.ResourcesCreated, job.DiggerJobSummary.ResourcesUpdated, job.DiggerJobSummary.ResourcesDeleted)
		opts := github.GithubCheckRunUpdateOptions{
			Status:     &status,
			Conclusion: &conclusion,
			Title:      &title,
			Summary:    &summary,
			Text:       &text,
			Actions:    utils.GetActionsForJob(job),
		}
		_, err = ghService.UpdateCheckRun(*job.CheckRunId, opts)
		slog.Error("Error updating PR status for job", "error", err)
	}
	return nil
}
