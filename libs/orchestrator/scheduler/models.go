package scheduler

import "fmt"

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
	default:
		return ":question:"
	}
}

type SerializedJob struct {
	DiggerJobId      string          `json:"digger_job_id"`
	Status           DiggerJobStatus `json:"status"`
	ProjectName      string          `json:"project_name"`
	JobString        []byte          `json:"job_string"`
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

func (s *SerializedJob) ResourcesSummaryString(isPlan bool) string {
	if isPlan && s.Status == DiggerJobSucceeded {
		return fmt.Sprintf(" [Resources: %v to create, %v to update, %v to delete]", s.ResourcesCreated, s.ResourcesUpdated, s.ResourcesDeleted)
	} else {
		return "..."
	}
}
