package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"log/slog"
)

func UpdateCheckStatusForBatch(gh utils.GithubClientProvider, batch *models.DiggerBatch) error {
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
		"newStatus", serializedBatch.ToStatusCheck())
	if isPlanBatch {
		prService.SetStatus(batch.PrNumber, serializedBatch.ToStatusCheck(), "digger/plan")
		prService.SetStatus(batch.PrNumber, "neutral", "digger/apply")
	} else {
		prService.SetStatus(batch.PrNumber, "success", "digger/plan")
		prService.SetStatus(batch.PrNumber, serializedBatch.ToStatusCheck(), "digger/apply")
	}
	return nil
}

func UpdateCheckStatusForJob(gh utils.GithubClientProvider, job *models.DiggerJob) error {
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
	status, err := models.GetStatusCheckForJob(job)
	if err != nil {
		return fmt.Errorf("could not get status check for job: %v", err)
	}
	slog.Debug("Updating PR status for job", "jobId", job.DiggerJobID, "status", status)
	if isPlan {
		prService.SetStatus(batch.PrNumber, status, jobSpec.GetProjectAlias()+"/plan")
		prService.SetStatus(batch.PrNumber, "neutral", jobSpec.GetProjectAlias()+"/apply")
	} else {
		//prService.SetStatus(batch.PrNumber, "success", jobSpec.GetProjectAlias()+"/plan")
		prService.SetStatus(batch.PrNumber, status, jobSpec.GetProjectAlias()+"/apply")
	}
	return nil
}
