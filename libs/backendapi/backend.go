package backendapi

import (
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/terraform_utils"
	"time"
)

type Api interface {
	ReportProject(repo string, projectName string, configuration string) error
	ReportProjectRun(repo string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error
	ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *terraform_utils.TerraformSummary, planJson string, PrCommentUrl string, terraformOutput string) (*scheduler.SerializedBatch, error)
	UploadJobArtefact(zipLocation string) (*int, *string, error)
	DownloadJobArtefact(downloadTo string) (*string, error)
}
