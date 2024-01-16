package models

import (
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type DiggerBatchStatus int8

const (
	BatchJobCreated     DiggerBatchStatus = 1
	BatchJobStarted     DiggerBatchStatus = 2
	BatchJobFailed      DiggerBatchStatus = 3
	BatchJobSucceeded   DiggerBatchStatus = 4
	BatchJobInvalidated DiggerBatchStatus = 5
)

type DiggerBatchType string

const (
	BatchTypePlan  DiggerBatchType = "plan"
	BatchTypeApply DiggerBatchType = "apply"
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

type DiggerBatch struct {
	ID                   uuid.UUID `gorm:"primary_key"`
	PrNumber             int
	CommentId            *int64
	Status               DiggerBatchStatus
	BranchName           string
	DiggerConfig         string
	GithubInstallationId int64
	RepoFullName         string
	RepoOwner            string
	RepoName             string
	BatchType            DiggerBatchType
}

type DiggerJob struct {
	gorm.Model
	DiggerJobId        string `gorm:"size:50,index:idx_digger_job_id"`
	Status             DiggerJobStatus
	Batch              *DiggerBatch
	BatchID            *string `gorm:"index:idx_digger_job_id"`
	DiggerJobSummary   *DiggerJobSummary
	DiggerJobSummaryId *uint `gorm:""`
	SerializedJob      []byte
	StatusUpdatedAt    time.Time
}

type DiggerJobSummary struct {
	gorm.Model
	ResourcesCreated uint
	ResourcesDeleted uint
	ResourcesUpdated uint
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

type SerializedJob struct {
	DiggerJobId      string
	Status           DiggerJobStatus
	ProjectName      string
	ResourcesCreated uint
	ResourcesDeleted uint
	resourcesUpdated uint
}

type SerializedBatch struct {
	ID           string
	PrNumber     int
	Status       DiggerBatchStatus
	BranchName   string
	RepoFullName string
	RepoOwner    string
	RepoName     string
	BatchType    DiggerBatchType
	Jobs         []SerializedJob
}

func (j *DiggerJob) MapToJsonStruct() interface{} {
	if j.DiggerJobSummary == nil {
		return SerializedJob{
			DiggerJobId: j.DiggerJobId,
			Status:      j.Status,
		}
	} else {
		return SerializedJob{
			DiggerJobId:      j.DiggerJobId,
			Status:           j.Status,
			ResourcesCreated: j.DiggerJobSummary.ResourcesCreated,
			resourcesUpdated: j.DiggerJobSummary.ResourcesUpdated,
			ResourcesDeleted: j.DiggerJobSummary.ResourcesDeleted,
		}
	}
}
func (b *DiggerBatch) MapToJsonStruct() (interface{}, error) {

	res := SerializedBatch{
		ID:           b.ID.String(),
		PrNumber:     b.PrNumber,
		Status:       b.Status,
		BranchName:   b.BranchName,
		RepoFullName: b.RepoFullName,
		RepoOwner:    b.RepoOwner,
		RepoName:     b.RepoName,
		BatchType:    b.BatchType,
	}

	serializedJobs := make([]SerializedJob, 0)
	jobs, err := DB.GetDiggerJobsForBatch(b.ID)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshall digger batch: %v", err)
	}
	for _, job := range jobs {
		serializedJobs = append(serializedJobs, job.MapToJsonStruct().(SerializedJob))
	}
	res.Jobs = serializedJobs
	return res, nil
}
