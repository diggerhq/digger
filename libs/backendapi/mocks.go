package backendapi

import (
	"time"

	"github.com/diggerhq/digger/libs/iac_utils"
	"github.com/diggerhq/digger/libs/scheduler"
)

type MockBackendApi struct{}

func (t MockBackendApi) ReportProject(namespace, projectName, configuration string) error {
	return nil
}

func (t MockBackendApi) ReportProjectRun(repo, projectName string, startedAt, endedAt time.Time, status, command, output string) error {
	return nil
}

func (t MockBackendApi) ReportProjectJobStatus(repo, projectName, jobId, status string, timestamp time.Time, summary *iac_utils.IacSummary, planJson, PrCommentUrl, PrCommentId, terraformOutput string, iacUtils iac_utils.IacUtils) (*scheduler.SerializedBatch, error) {
	return nil, nil
}

func (t MockBackendApi) UploadJobArtefact(zipLocation string) (*int, *string, error) {
	return nil, nil, nil
}

func (t MockBackendApi) DownloadJobArtefact(downloadTo string) (*string, error) {
	return nil, nil
}
