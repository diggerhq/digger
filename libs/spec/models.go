package spec

import "github.com/diggerhq/digger/libs/orchestrator"

type ReporterSpec struct {
	Reporting_strategy string `json:"reporting_strategy"`
}

type LockSpec struct {
	LockType string `json:"lock_type"`
}

type BackendSpec struct {
	BackendType             string `json:"backend_type"`
	BackendHostname         string `json:"backend_hostname"`
	BackendOrganisationName string `json:"backend_organisation_hostname"`
	BackendJobToken         string `json:"backend_job_token"`
}

type Spec struct {
	Job      orchestrator.JobJson `json:"job"`
	reporter ReporterSpec         `json:"reporter"`
	lock     LockSpec             `json:"lock"`
	backend  BackendSpec          `json:"backend"`
}
