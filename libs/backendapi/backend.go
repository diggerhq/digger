package backendapi

import (
	"time"

	"github.com/go-substrate/strate/libs/iac_utils"
	"github.com/go-substrate/strate/libs/scheduler"
)

type Api interface {
	ReportProject(repo string, projectName string, configuration string) error
	ReportProjectRun(repo string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error
	ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *iac_utils.IacSummary, planJson string, PrCommentUrl string, terraformOutput string, iacUtils iac_utils.IacUtils) (*scheduler.SerializedBatch, error)
	UploadJobArtefact(zipLocation string) (*int, *string, error)
	DownloadJobArtefact(downloadTo string) (*string, error)
}
