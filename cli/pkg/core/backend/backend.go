package backend

import (
	"github.com/diggerhq/digger/cli/pkg/core/execution"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"time"
)

type Api interface {
	ReportProject(repo string, projectName string, configuration string) error
	ReportProjectRun(repo string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error
	ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, planResult *execution.DiggerExecutorPlanResult) (*scheduler.SerializedBatch, error)
}
