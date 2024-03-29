package models

import (
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"gorm.io/gorm"
)

type DiggerRunStatus string

const (
	RunQueued          DiggerRunStatus = "Queued"
	RunSucceeded       DiggerRunStatus = "Succeeded"
	RunFailed          DiggerRunStatus = "Failed"
	RunPendingApproval DiggerRunStatus = "Pending Approval"
)

type RunType string

const (
	PlanAndApply RunType = "Plan and Apply"
	PlanOnly     RunType = "Plan Only"
)

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
	DiggerRunStageID   string `gorm:"size:50,index:idx_digger_run_stage_id"`
	ProjectName        string
	Status             orchestrator_scheduler.DiggerJobStatus
	Run                *DiggerRun
	RunID              uint `gorm:"index:idx_digger_run_stage_id"`
	DiggerJobSummary   DiggerJobSummary
	DiggerJobSummaryID uint
	SerializedJobSpec  []byte
	WorkflowFile       string
	WorkflowRunUrl     *string
}

func (r *DiggerRunStage) MapToJsonStruct() (interface{}, error) {
	return orchestrator_scheduler.SerializedJob{
		DiggerJobId:      r.DiggerRunStageID,
		Status:           r.Status,
		JobString:        r.SerializedJobSpec,
		ProjectName:      r.ProjectName,
		WorkflowRunUrl:   r.WorkflowRunUrl,
		ResourcesCreated: r.DiggerJobSummary.ResourcesCreated,
		ResourcesUpdated: r.DiggerJobSummary.ResourcesUpdated,
		ResourcesDeleted: r.DiggerJobSummary.ResourcesDeleted,
	}, nil
}
