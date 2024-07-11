package spec

import (
	"github.com/diggerhq/digger/libs/scheduler"
)

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
	Actor                    string `json:"actor"`
	RepoFullname             string `json:"repo_full_name"`
	RepoName                 string `json:"repo_name"`
	RepoOwner                string `json:"repo_owner"`
	VcsType                  string `json:"vcs_type"`
	WorkflowFile             string `json:"workflow_file"`
	GithubEnterpriseHostname string `json:"github_enterprise_hostname"`
}

type PolicySpec struct {
	PolicyType string `json:"policy_type"`
}

type SpecType string

const SpecTypePullRequestJob SpecType = "pull_request_job"
const SpecTypeManualJob SpecType = "manual_job"
const SpecTypeDriftJob SpecType = "drift_job"

type Spec struct {
	// TODO: replace these three to be nested into one of the other specs
	SpecType  SpecType `json:"spec_type"`
	JobId     string   `json:"job_id"`
	CommentId string   `json:"comment_id"`
	RunName   string   `json:"run_name"`

	Job      scheduler.JobJson `json:"job"`
	Reporter ReporterSpec      `json:"reporter"`
	Lock     LockSpec          `json:"lock"`
	Backend  BackendSpec       `json:"backend"`
	VCS      VcsSpec           `json:"vcs"`
	Policy   PolicySpec        `json:"policy_provider"`
}
