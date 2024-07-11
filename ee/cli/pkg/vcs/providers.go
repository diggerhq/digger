package vcs

import (
	"fmt"
	github2 "github.com/diggerhq/digger/ee/cli/pkg/github"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/gitlab"
	"github.com/diggerhq/digger/libs/spec"
	"os"
)

type VCSProviderAdvanced struct{}

func (v VCSProviderAdvanced) GetPrService(vcsSpec spec.VcsSpec) (ci.PullRequestService, error) {
	switch vcsSpec.VcsType {
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		return github2.GithubServiceProviderAdvanced{}.NewService(token, vcsSpec.RepoName, vcsSpec.RepoOwner)
	case "gitlab":
		token := os.Getenv("GITLAB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get gitlab service: GITLAB_TOKEN not specified")
		}
		context, err := gitlab.ParseGitLabContext()
		if err != nil {
			return nil, fmt.Errorf("failed to get gitlab service, could not parse context: %v", err)
		}
		baseUrl := os.Getenv("DIGGER_GITLAB_BASE_URL")
		return gitlab.NewGitLabService(token, context, baseUrl)
	default:
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}
}

func (v VCSProviderAdvanced) GetOrgService(vcsSpec spec.VcsSpec) (ci.OrgService, error) {
	switch vcsSpec.VcsType {
	case "gitlab":
		token := os.Getenv("GITLAB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get gitlab service: GITLAB_TOKEN not specified")
		}
		context, err := gitlab.ParseGitLabContext()
		if err != nil {
			return nil, fmt.Errorf("failed to get gitlab service, could not parse context: %v", err)
		}
		baseUrl := os.Getenv("DIGGER_GITLAB_BASE_URL")
		return gitlab.NewGitLabService(token, context, baseUrl)
	default:
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}
}
