package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type DiggerJobStatus int8

const (
	DiggerJobCreated   DiggerJobStatus = 1
	DiggerJobTriggered DiggerJobStatus = 2
	DiggerJobFailed    DiggerJobStatus = 3
	DiggerJobStarted   DiggerJobStatus = 4
	DiggerJobSucceeded DiggerJobStatus = 5
)

type DiggerJobParentLink struct {
	gorm.Model
	DiggerJobId       string `gorm:"size:50,index:idx_digger_job_id"`
	ParentDiggerJobId string `gorm:"size:50,index:idx_parent_digger_job_id"`
}

type DiggerJob struct {
	gorm.Model
	DiggerJobId     string `gorm:"size:50,index:idx_digger_job_id"`
	Status          DiggerJobStatus
	BatchId         uuid.UUID `gorm:"index:idx_batch_id"`
	SerializedJob   []byte
	BranchName      string
	StatusUpdatedAt time.Time
}

type DiggerJobLinkStatus int8

const (
	DiggerJobLinkCreated   DiggerJobLinkStatus = 1
	DiggerJobLinkSucceeded DiggerJobLinkStatus = 2
)

// GithubDiggerJobLink links GitHub Workflow Job id to Digger's Job Id
type GithubDiggerJobLink struct {
	gorm.Model
	DiggerJobId         string `gorm:"size:50,index:idx_digger_job_id"`
	RepoFullName        string
	GithubJobId         int64 `gorm:"index:idx_github_job_id"`
	GithubWorkflowRunId int64
	Status              DiggerJobLinkStatus
}
