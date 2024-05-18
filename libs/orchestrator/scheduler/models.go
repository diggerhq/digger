package scheduler

import (
	"fmt"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/goccy/go-json"
	"log"
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
	DiggerJobCreated      DiggerJobStatus = 1
	DiggerJobTriggered    DiggerJobStatus = 2
	DiggerJobFailed       DiggerJobStatus = 3
	DiggerJobStarted      DiggerJobStatus = 4
	DiggerJobSucceeded    DiggerJobStatus = 5
	DiggerJobQueuedForRun DiggerJobStatus = 6
)

func (d *DiggerJobStatus) ToString() string {
	switch *d {
	case DiggerJobSucceeded:
		return "succeeded"
	case DiggerJobStarted:
		return "running"
	case DiggerJobFailed:
		return "failed"
	case DiggerJobTriggered:
		return "running"
	case DiggerJobCreated:
		return "created"
	case DiggerJobQueuedForRun:
		return "created"
	default:
		return "unknown status"
	}
}

func (d *DiggerJobStatus) ToEmoji() string {
	switch *d {
	case DiggerJobSucceeded:
		return ":white_check_mark:"
	case DiggerJobStarted:
		return ":arrows_counterclockwise:"
	case DiggerJobFailed:
		return ":x:"
	case DiggerJobTriggered:
		return ":arrows_counterclockwise:"
	case DiggerJobCreated:
		return ":clock11:"
	case DiggerJobQueuedForRun:
		return ":clock11:"
	default:
		return ":question:"
	}
}

type SerializedJob struct {
	DiggerJobId      string          `json:"digger_job_id"`
	Status           DiggerJobStatus `json:"status"`
	ProjectName      string          `json:"project_name"`
	JobString        []byte          `json:"job_string"`
	PRCommentUrl     string          `json:"pr_comment_url"`
	WorkflowRunUrl   *string         `json:"workflow_run_url"`
	ResourcesCreated uint            `json:"resources_created"`
	ResourcesDeleted uint            `json:"resources_deleted"`
	ResourcesUpdated uint            `json:"resources_updated"`
}

type SerializedBatch struct {
	ID           string            `json:"id"`
	PrNumber     int               `json:"pr_number"`
	Status       DiggerBatchStatus `json:"status"`
	BranchName   string            `json:"branch_name"`
	RepoFullName string            `json:"repo_full_name"`
	RepoOwner    string            `json:"repo_owner"`
	RepoName     string            `json:"repo_name"`
	BatchType    DiggerBatchType   `json:"batch_type"`
	Jobs         []SerializedJob   `json:"jobs"`
}

func (b *SerializedBatch) IsPlan() (bool, error) {
	// TODO: Introduce a batch-level field to check for is plan or apply
	jobSpecs, err := GetJobSpecs(b.Jobs)
	if err != nil {
		log.Printf("error while fetching job specs: %v", err)
		return false, fmt.Errorf("error while fetching job specs: %v", err)
	}
	return orchestrator.IsPlanJobSpecs(jobSpecs), nil
}

func (b *SerializedBatch) IsApply() (bool, error) {
	jobSpecs, err := GetJobSpecs(b.Jobs)
	if err != nil {
		log.Printf("error while fetching job specs: %v", err)
		return false, fmt.Errorf("error while fetching job specs: %v", err)
	}
	return orchestrator.IsPlanJobSpecs(jobSpecs), nil
}

func (b *SerializedBatch) ToStatusCheck() string {
	switch b.Status {
	case BatchJobCreated:
		return "pending"
	case BatchJobInvalidated:
		return "failure"
	case BatchJobFailed:
		return "success"
	case BatchJobSucceeded:
		return "success"
	default:
		return "pending"
	}
}

func (s *SerializedJob) ResourcesSummaryString(isPlan bool) string {
	if !isPlan {
		return ""
	}

	if s.Status == DiggerJobSucceeded {
		return fmt.Sprintf(" [Resources: %v to create, %v to update, %v to delete]", s.ResourcesCreated, s.ResourcesUpdated, s.ResourcesDeleted)
	} else {
		return "..."
	}
}

func GetJobSpecs(jobs []SerializedJob) ([]orchestrator.JobJson, error) {
	jobSpecs := make([]orchestrator.JobJson, 0)
	for _, job := range jobs {
		var jobSpec orchestrator.JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job")
			return nil, err
		}
		jobSpecs = append(jobSpecs, jobSpec)
	}
	return jobSpecs, nil
}
