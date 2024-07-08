package ci_backends

import (
	"fmt"
	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/xanzy/go-gitlab"
	"log"
	"os"
)

type EEBackendProvider struct{}

func (b EEBackendProvider) GetCiBackend(options ci_backends.CiBackendOptions) (ci_backends.CiBackend, error) {
	ciBackendType := os.Getenv("DIGGER_CI_BACKEND")
	switch ciBackendType {
	case "github_actions", "":
		return ci_backends.DefaultBackendProvider{}.GetCiBackend(options)
	case "gitlab_pipelines":
		token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("missing environment variable: DIGGER_GITLAB_ACCESS_TOKEN")
		}
		client, err := gitlab.NewClient(token)
		if err != nil {
			return nil, fmt.Errorf("could not create gitlab client: %v", err)
		}
		return GitlabPipelineCI{
			Client:                      client,
			GitlabProjectId:             options.GitlabProjectId,
			GitlabmergeRequestEventName: options.GitlabmergeRequestEventName,
			GitlabCIPipelineID:          options.GitlabCIPipelineID,
			GitlabCIPipelineIID:         options.GitlabCIPipelineIID,
			GitlabCIMergeRequestID:      options.GitlabCIMergeRequestID,
			GitlabCIMergeRequestIID:     options.GitlabCIMergeRequestIID,
			GitlabCIProjectName:         options.GitlabCIProjectName,
			GitlabciprojectNamespace:    options.GitlabciprojectNamespace,
			GitlabciprojectId:           options.GitlabciprojectId,
			GitlabciprojectNamespaceId:  options.GitlabciprojectNamespaceId,
			GitlabDiscussionId:          options.GitlabDiscussionId,
		}, nil
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
