package ci_backends

import (
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/spec"
)

type CiBackend interface {
	TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error
}

type JenkinsCi struct{}

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
