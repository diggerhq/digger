package backend

import "time"

type JobSummary struct {
	ResourcesCreated uint
	ResourcesUpdated uint
	ResourcesDeleted uint
}

type Api interface {
	ReportProject(repo string, projectName string, configuration string) error
	ReportProjectRun(repo string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error
	ReportProjectJobStatus(repo string, projectName string, jobId string, status string, timestamp time.Time, summary *JobSummary) error
}

func (j *JobSummary) ToJson() map[string]interface{} {
	if j == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"resources_created": j.ResourcesCreated,
		"resources_updated": j.ResourcesUpdated,
		"resources_deleted": j.ResourcesDeleted,
	}
}
