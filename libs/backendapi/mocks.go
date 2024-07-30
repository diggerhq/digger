package backendapi

import (
	"github.com/diggerhq/digger/libs/execution"
	"github.com/diggerhq/digger/libs/scheduler"
	"time"
)

type MockBackendApi struct {
}

func (t MockBackendApi) ReportProject(namespace string, projectName string, configuration string) error {
	return nil
}

func (t MockBackendApi) ReportProjectRun(repo string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error {
	return nil
}

func (t MockBackendApi) ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *execution.DiggerExecutorPlanResult, PrCommentUrl string, terraformOutput string) (*scheduler.SerializedBatch, error) {
	return nil, nil
}

func (t MockBackendApi) UploadJobArtefact(zipLocation string) (*int, *string, error) {
	return nil, nil, nil
}

func (t MockBackendApi) DownloadJobArtefact(downloadTo string) (*string, error) {
	return nil, nil
}
