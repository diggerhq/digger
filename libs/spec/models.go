package spec

import "github.com/diggerhq/digger/libs/orchestrator"

type StepJson struct {
	Action    string   `json:"action"`
	Value     string   `json:"value"`
	ExtraArgs []string `json:"extraArgs"`
	Shell     string   `json:"shell"`
}

type StageJson struct {
	Steps []StepJson `json:"steps"`
}

type ReporterSpec struct {
	reporting_strategy string `json:"reporting_strategy"`
}

type LockSpec struct {
	lockType string `json:"lock_type"`
}

type BackendSpec struct {
	backendType             string `json:"backend_type"`
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
