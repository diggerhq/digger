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
	DiggerJobID        string `gorm:"size:50,index:idx_digger_job_id"`
	Status             orchestrator_scheduler.DiggerJobStatus
	Batch              *DiggerBatch
	BatchID            *string `gorm:"index:idx_digger_job_id"`
	PRCommentUrl       string
	DiggerJobSummary   DiggerJobSummary
	DiggerJobSummaryID uint
	SerializedJobSpec  []byte
	WorkflowFile       string
	WorkflowRunUrl     *string
	StatusUpdatedAt    time.Time
}

type DiggerJobSummary struct {
	gorm.Model
	ResourcesCreated uint
	ResourcesDeleted uint
	ResourcesUpdated uint
}

// These tokens will be pre
type JobToken struct {
	gorm.Model
	Value          string `gorm:"uniqueJobTokenIndex:idx_token"`
	Expiry         time.Time
	OrganisationID uint
	Organisation   Organisation
	Type           string // AccessTokenType starts with j:
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

func (j *DiggerJob) MapToJsonStruct() (interface{}, error) {
	var job orchestrator.JobJson
	err := json.Unmarshal(j.SerializedJobSpec, &job)
	if err != nil {
		log.Printf("Failed to convert unmarshall Serialized job, %v", err)
	}
	return orchestrator_scheduler.SerializedJob{
		DiggerJobId:      j.DiggerJobID,
		Status:           j.Status,
		JobString:        j.SerializedJobSpec,
		ProjectName:      job.ProjectName,
		WorkflowRunUrl:   j.WorkflowRunUrl,
		PRCommentUrl:     j.PRCommentUrl,
		ResourcesCreated: j.DiggerJobSummary.ResourcesCreated,
		ResourcesUpdated: j.DiggerJobSummary.ResourcesUpdated,
		ResourcesDeleted: j.DiggerJobSummary.ResourcesDeleted,
	}, nil
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
		return nil, fmt.Errorf("could not unmarshall digger batch: %v", err)
	}
	for _, job := range jobs {
		jobJson, err := job.MapToJsonStruct()
		if err != nil {
			return nil, fmt.Errorf("error mapping job to struct (ID: %v); %v", job.ID, err)
		}
		serializedJobs = append(serializedJobs, jobJson.(orchestrator_scheduler.SerializedJob))
	}
	res.Jobs = serializedJobs
	return res, nil
}
