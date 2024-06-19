package ci_backends

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
)

type CiBackend interface {
	TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error
}

type JenkinsCi struct{}

type CiBackendOptions struct {
	GithubClientProvider utils.GithubClientProvider
	GithubInstallationId int64
	RepoFullName         string
	RepoOwner            string
	RepoName             string
}
