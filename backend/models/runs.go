package models

import (
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"gorm.io/gorm"
	"log"
	"time"
)

type DiggerRunStatus string

const (
	RunQueued          DiggerRunStatus = "Queued"
	RunPendingPlan     DiggerRunStatus = "Pending Plan"
	RunPlanning        DiggerRunStatus = "Running Plan"
	RunPendingApproval DiggerRunStatus = "Pending Approval"
	RunApproved        DiggerRunStatus = "Approved"
	RunPendingApply    DiggerRunStatus = "Pending Apply"
	RunApplying        DiggerRunStatus = "Running Apply"
	RunSucceeded       DiggerRunStatus = "Succeeded"
	RunFailed          DiggerRunStatus = "Failed"
)

type RunType string

const (
	PlanAndApply RunType = "Plan and Apply"
	PlanOnly     RunType = "Plan Only"
)

type DiggerRunQueueItem struct {
	gorm.Model
	ProjectId   uint `gorm:"index:idx_digger_run_queue_project_id"`
	Project     *Project
	DiggerRunId uint `gorm:"index:idx_digger_run_queue_run_id"`
	DiggerRun   DiggerRun
	time        time.Time
}

type DiggerRun struct {
	gorm.Model
	Triggertype          string // pr_merge, manual_invocation, push_to_trunk
	PrNumber             *int
	Status               DiggerRunStatus
	CommitId             string
	DiggerConfig         string
	GithubInstallationId int64
	RepoId               uint
	Repo                 *Repo
	Project              *Project
	ProjectID            uint
	RunType              RunType
}

type DiggerRunStage struct {
	gorm.Model
	Run     *DiggerRun
	RunID   uint `gorm:"index:idx_digger_run_stage_id"`
	Batch   *DiggerBatch
	BatchID *string `gorm:"index:idx_digger_job_id"`
}

type SerializedRunStage struct {
	DiggerJobId      string                                 `json:"digger_job_id"`
	Status           orchestrator_scheduler.DiggerJobStatus `json:"status"`
	ProjectName      string                                 `json:"project_name"`
	WorkflowRunUrl   *string                                `json:"workflow_run_url"`
	ResourcesCreated uint                                   `json:"resources_created"`
	ResourcesDeleted uint                                   `json:"resources_deleted"`
	ResourcesUpdated uint                                   `json:"resources_updated"`
}

func (r *DiggerRunStage) MapToJsonStruct() (interface{}, error) {
	job, err := DB.GetDiggerJobFromRunStage(*r)
	if err != nil {
		log.Printf("Could not retrive job from run")
		return nil, err
	}

	return SerializedRunStage{
		DiggerJobId:      job.DiggerJobID,
		Status:           job.Status,
		ProjectName:      r.Run.Project.Name,
		WorkflowRunUrl:   job.WorkflowRunUrl,
		ResourcesCreated: job.DiggerJobSummary.ResourcesCreated,
		ResourcesUpdated: job.DiggerJobSummary.ResourcesUpdated,
		ResourcesDeleted: job.DiggerJobSummary.ResourcesDeleted,
	}, nil
}
