package models

import (
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
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
	DiggerRunId uint `gorm:"index:idx_digger_run_queue_run_id"`
	DiggerRun   DiggerRun
	ProjectId   uint
	Project     *Project
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
	ProjectName          string
	RunType              RunType
	PlanStage            DiggerRunStage
	PlanStageId          *uint
	ApplyStage           DiggerRunStage
	ApplyStageId         *uint
	IsApproved           bool
	ApprovalAuthor       string
	ApprovalDate         time.Time
}

type DiggerRunStage struct {
	gorm.Model
	Batch   *DiggerBatch
	BatchID *string `gorm:"index:idx_digger_run_batch_id"`
}

type SerializedRunStage struct {
	//DiggerRunId           uint                                   `json:"digger_run_id"`
	DiggerJobId           string                                 `json:"digger_job_id"`
	Status                orchestrator_scheduler.DiggerJobStatus `json:"status"`
	ProjectName           string                                 `json:"project_name"`
	WorkflowRunUrl        *string                                `json:"workflow_run_url"`
	ResourcesCreated      uint                                   `json:"resources_created"`
	ResourcesDeleted      uint                                   `json:"resources_deleted"`
	ResourcesUpdated      uint                                   `json:"resources_updated"`
	LastActivityTimeStamp string                                 `json:"last_activity_timestamp"`
}

func (r *DiggerRun) MapToJsonStruct() (interface{}, error) {
	planStage, err := r.PlanStage.MapToJsonStruct()
	if err != nil {
		log.Printf("error serializing run: %v", err)
		return nil, err
	}

	applyStage, err := r.ApplyStage.MapToJsonStruct()
	if err != nil {
		log.Printf("error serializing run: %v", err)
		return nil, err
	}

	x := struct {
		Id                    uint               `json:"id"`
		Status                string             `json:"status"`
		Type                  string             `json:"type"`
		ApprovalAuthor        string             `json:"approval_author"`
		ApprovalDate          string             `json:"approval_date"`
		LastActivityTimeStamp string             `json:"last_activity_time_stamp"`
		PlanStage             SerializedRunStage `json:"plan_stage"`
		ApplyStage            SerializedRunStage `json:"apply_stage"`
		IsApproved            bool               `json:"is_approved"`
	}{
		Id:                    r.ID,
		Status:                string(r.Status),
		Type:                  string(r.RunType),
		LastActivityTimeStamp: r.UpdatedAt.String(),
		PlanStage:             *planStage,
		ApplyStage:            *applyStage,
		IsApproved:            r.IsApproved,
		ApprovalAuthor:        r.ApprovalAuthor,
		ApprovalDate:          r.ApprovalDate.String(),
	}

	return x, nil
}

func (r DiggerRunStage) MapToJsonStruct() (*SerializedRunStage, error) {
	job, err := DB.GetDiggerJobFromRunStage(r)
	if err != nil {
		log.Printf("Could not retrive job from run")
		return nil, err
	}

	return &SerializedRunStage{
		DiggerJobId: job.DiggerJobID,
		Status:      job.Status,
		//ProjectName:      r.Run.ProjectName,
		WorkflowRunUrl:        job.WorkflowRunUrl,
		ResourcesCreated:      job.DiggerJobSummary.ResourcesCreated,
		ResourcesUpdated:      job.DiggerJobSummary.ResourcesUpdated,
		ResourcesDeleted:      job.DiggerJobSummary.ResourcesDeleted,
		LastActivityTimeStamp: r.UpdatedAt.String(),
	}, nil
}
