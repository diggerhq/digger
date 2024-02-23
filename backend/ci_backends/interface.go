package ci_backends

import (
	"github.com/diggerhq/digger/backend/models"
	"os"
)

type CiBackend interface {
	TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error
}

type JenkinsCi struct{}

func GetCiBackend() {
	ciBackend := os.Getenv("CI_BACKEND")
	switch ciBackend {

	default:
	}

}
