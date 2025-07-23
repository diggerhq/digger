package backendapi

import (
	"time"

	"github.com/diggerhq/digger/libs/iac_utils"
	"github.com/diggerhq/digger/libs/scheduler"
)

type Api interface {
	ReportProject(repo, projectName, configuration string) error
	ReportProjectRun(repo, projectName string, startedAt, endedAt time.Time, status, command, output string) error
	ReportProjectJobStatus(repo, projectName, jobId, status string, timestamp time.Time, summary *iac_utils.IacSummary, planJson, PrCommentUrl, PrCommentId, terraformOutput string, iacUtils iac_utils.IacUtils) (*scheduler.SerializedBatch, error)
	UploadJobArtefact(zipLocation string) (*int, *string, error)
	DownloadJobArtefact(downloadTo string) (*string, error)
}
