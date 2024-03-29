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
	Run   *DiggerRun
	RunID uint `gorm:"index:idx_digger_run_stage_id"`
	Job   *DiggerJob
	JobID uint
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
	return SerializedRunStage{
		DiggerJobId:      r.Job.DiggerJobID,
		Status:           r.Job.Status,
		ProjectName:      r.Run.Project.Name,
		WorkflowRunUrl:   r.Job.WorkflowRunUrl,
		ResourcesCreated: r.Job.DiggerJobSummary.ResourcesCreated,
		ResourcesUpdated: r.Job.DiggerJobSummary.ResourcesUpdated,
		ResourcesDeleted: r.Job.DiggerJobSummary.ResourcesDeleted,
	}, nil
}
