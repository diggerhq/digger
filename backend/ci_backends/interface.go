package ci_backends

import (
	"fmt"
	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"log"
	"os"
)

type CiBackend interface {
	TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error
}

type JenkinsCi struct{}

type CiBackendOptions struct {
	GithubInstallationId int64
	RepoFullName         string
	RepoOwner            string
	RepoName             string
}

func GetCiBackend(options CiBackendOptions) (CiBackend, error) {
	ciBackendType := os.Getenv("CI_BACKEND")
	switch ciBackendType {
	case "github_actions", "":
		client, _, err := utils.GetGithubClient(&utils.DiggerGithubRealClientProvider{}, options.GithubInstallationId, options.RepoFullName)
		if err != nil {
			log.Printf("GetCiBackend: could not get buildkite ci: %v", err)
			return nil, fmt.Errorf("could not get buildkite ci: %v", err)
		}
		backend := &GithubActionCi{
			Client: client,
		}
		return backend, nil
	case "buildkite":
		token := os.Getenv("BUILDKITE_TOKEN")
		org := os.Getenv("BUILDKITE_ORG")
		pipeline := os.Getenv("BUILDKITE_PIPELINE")
		if token == "" || org == "" || pipeline == "" {
			return nil, fmt.Errorf("missing environment variable: required BUILDKITE_TOKEN, BUILDKITE_ORG, BUILDKITE_PIPELINE")
		}
		bconfig, err := buildkite.NewTokenConfig(token, false)
		if err != nil {
			log.Printf("could not create buildkite client: %v", err)
			return nil, fmt.Errorf("could not create buildkite client: %v", err)
		}
		buildkite := buildkite.NewClient(bconfig.Client())
		ciBackend := &BuildkiteCi{Org: org, Pipeline: pipeline, Client: *buildkite}
		return ciBackend, nil
	}
	return nil, fmt.Errorf("unkown ci system: %v", ciBackendType)
}
