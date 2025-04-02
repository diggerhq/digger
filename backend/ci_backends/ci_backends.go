package ci_backends

import (
	"github.com/go-substrate/strate/backend/utils"
	"github.com/go-substrate/strate/libs/spec"
)

type CiBackend interface {
	TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error
	GetWorkflowUrl(spec spec.Spec) (string, error)
}

type CiBackendOptions struct {
	GithubClientProvider        utils.GithubClientProvider
	GithubInstallationId        int64
	GithubAppId                 int64
	GitlabProjectId             int
	GitlabmergeRequestEventName string
	GitlabCIPipelineID          string
	GitlabCIPipelineIID         int
	GitlabCIMergeRequestID      int
	GitlabCIMergeRequestIID     int
	GitlabCIProjectName         string
	GitlabciprojectNamespace    string
	GitlabciprojectId           int
	GitlabciprojectNamespaceId  int
	GitlabDiscussionId          string
	RepoFullName                string
	RepoOwner                   string
	RepoName                    string
}
