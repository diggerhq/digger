package spec

import "github.com/diggerhq/digger/libs/orchestrator"

type ReporterSpec struct {
	ReporterType      string `json:"reporter_type"`
	ReportingStrategy string `json:"reporting_strategy"`
}

type LockSpec struct {
	LockType     string `json:"lock_type"`
	LockProvider string `json:"lock_provider"`
}

type BackendSpec struct {
	BackendType             string `json:"backend_type"`
	BackendHostname         string `json:"backend_hostname"`
	BackendOrganisationName string `json:"backend_organisation_hostname"`
	BackendJobToken         string `json:"backend_job_token"`
}

type VcsSpec struct {
	RepoName  string `json:"repo_name"`
	RepoOwner string `json:"repo_owner"`
	VcsType   string `json:"vcs_type"`
}

type PolicySpec struct {
	PolicyType string `json:"policy_type"`
}

type Spec struct {
	Job      orchestrator.JobJson `json:"job"`
	Reporter ReporterSpec         `json:"reporter"`
	Lock     LockSpec             `json:"lock"`
	Backend  BackendSpec          `json:"backend"`
	VCS      VcsSpec              `json:"vcs"`
	Policy   PolicySpec           `json:"policyProvider"`
}
