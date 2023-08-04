package backend

import "time"

type Api interface {
	ReportProject(namespace string, projectName string, configuration string) error
	ReportProjectRun(namespace string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error
}
