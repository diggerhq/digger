package models

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log"
	"time"
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
	Status               orchestrator_scheduler.DiggerBatchStatus
	BranchName           string
	DiggerConfig         string
	GithubInstallationId int64
	RepoFullName         string
	RepoOwner            string
	RepoName             string
	BatchType            orchestrator_scheduler.DiggerBatchType
}

type DiggerJob struct {
	gorm.Model
	DiggerJobId        string `gorm:"size:50,index:idx_digger_job_id"`
	Status             orchestrator_scheduler.DiggerJobStatus
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

func (j *DiggerJob) MapToJsonStruct() interface{} {
	if j.DiggerJobSummary == nil {
		return orchestrator_scheduler.SerializedJob{
			DiggerJobId: j.DiggerJobId,
			Status:      j.Status,
		}
	} else {
		var job orchestrator.JobJson
		err := json.Unmarshal(j.SerializedJob, &job)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job")
		}
		return orchestrator_scheduler.SerializedJob{
			DiggerJobId:      j.DiggerJobId,
			Status:           j.Status,
			JobString:        j.SerializedJob,
			ProjectName:      job.ProjectName,
			ResourcesCreated: j.DiggerJobSummary.ResourcesCreated,
			ResourcesUpdated: j.DiggerJobSummary.ResourcesUpdated,
			ResourcesDeleted: j.DiggerJobSummary.ResourcesDeleted,
		}
	}
}
func (b *DiggerBatch) MapToJsonStruct() (interface{}, error) {
	res := orchestrator_scheduler.SerializedBatch{
		ID:           b.ID.String(),
		PrNumber:     b.PrNumber,
		Status:       b.Status,
		BranchName:   b.BranchName,
		RepoFullName: b.RepoFullName,
		RepoOwner:    b.RepoOwner,
		RepoName:     b.RepoName,
		BatchType:    b.BatchType,
	}

	serializedJobs := make([]orchestrator_scheduler.SerializedJob, 0)
	jobs, err := DB.GetDiggerJobsForBatch(b.ID)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshall digger batch: %v", err)
	}
	for _, job := range jobs {
		serializedJobs = append(serializedJobs, job.MapToJsonStruct().(orchestrator_scheduler.SerializedJob))
	}
	res.Jobs = serializedJobs
	return res, nil
}
