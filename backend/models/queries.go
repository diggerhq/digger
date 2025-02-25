package models

import "time"

type JobQueryResult struct {
	ID              uint       `gorm:"column:id"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
	DiggerJobID     string     `gorm:"column:digger_job_id"`
	Status          string     `gorm:"column:status"`
	WorkflowRunURL  string     `gorm:"column:workflow_run_url"`
	WorkflowFile    string     `gorm:"column:workflow_file"`
	TerraformOutput string     `gorm:"column:terraform_output"`
	PRNumber        int        `gorm:"column:pr_number"`
	RepoFullName    string     `gorm:"column:repo_full_name"`
	BranchName      string     `gorm:"column:branch_name"`
}
