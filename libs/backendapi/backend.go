package backendapi

import (
	"github.com/diggerhq/digger/libs/execution"
	"github.com/diggerhq/digger/libs/scheduler"
	"time"
)

type Api interface {
	ReportProject(repo string, projectName string, configuration string) error
	ReportProjectRun(repo string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error
	ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *execution.DiggerExecutorPlanResult, PrCommentUrl string, terraformOutput string) (*scheduler.SerializedBatch, error)
}
