package dbmodels

import (
	"encoding/json"
	"fmt"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/model"
	"log"
)

type DiggerVCSType string

const DiggerVCSGithub DiggerVCSType = "github"
const DiggerVCSGitlab DiggerVCSType = "gitlab"

type DiggerJobLinkStatus int8

type BatchEventType string

const DiggerBatchMergeEvent = "merge_event"
const DiggerBatchPullRequestEvent = "pull_request_event"
const DiggerBatchDriftEvent = "drift_event"
const DiggerBatchManualTriggerEvent = "manual_trigger"

const (
	DiggerJobLinkCreated   DiggerJobLinkStatus = 1
	DiggerJobLinkSucceeded DiggerJobLinkStatus = 2
)

func JobToJsonStruct(j model.DiggerJob) (orchestrator_scheduler.SerializedJob, error) {
	var job orchestrator_scheduler.JobJson
	err := json.Unmarshal(j.JobSpec, &job)
	if err != nil {
		log.Printf("Failed to convert unmarshall Serialized job, %v", err)
	}

	return orchestrator_scheduler.SerializedJob{
		DiggerJobId:      j.DiggerJobID,
		Status:           orchestrator_scheduler.DiggerJobStatus(j.Status),
		JobString:        j.JobSpec,
		PlanFootprint:    j.PlanFootprint,
		ProjectName:      job.ProjectName,
		WorkflowRunUrl:   &j.WorkflowRunURL,
		PRCommentUrl:     j.PrCommentURL,
		ResourcesCreated: 0, // todo: fetch from summary
		ResourcesUpdated: 0,
		ResourcesDeleted: 0,
	}, nil
}

func BatchToJsonStruct(b model.DiggerBatch) (orchestrator_scheduler.SerializedBatch, error) {
	res := orchestrator_scheduler.SerializedBatch{
		ID:           b.ID,
		PrNumber:     int(b.PrNumber),
		Status:       orchestrator_scheduler.DiggerBatchStatus(b.Status),
		BranchName:   b.BranchName,
		RepoFullName: b.RepoFullName,
		RepoOwner:    b.RepoOwner,
		RepoName:     b.RepoName,
		BatchType:    orchestrator_scheduler.DiggerCommand(b.BatchType),
	}

	serializedJobs := make([]orchestrator_scheduler.SerializedJob, 0)
	jobs, err := DB.GetDiggerJobsForBatch(b.ID)
	if err != nil {
		return res, fmt.Errorf("could not unmarshall digger batch: %v", err)
	}
	for _, job := range jobs {
		jobJson, err := JobToJsonStruct(job)
		if err != nil {
			return res, fmt.Errorf("error mapping job to struct (ID: %v); %v", job.ID, err)
		}
		serializedJobs = append(serializedJobs, jobJson)
	}
	res.Jobs = serializedJobs
	return res, nil
}
