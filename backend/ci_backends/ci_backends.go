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
	GithubClientProvider utils.GithubClientProvider
	GithubInstallationId int64
	RepoFullName         string
	RepoOwner            string
	RepoName             string
}
