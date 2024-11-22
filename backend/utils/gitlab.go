package utils

import (
	"fmt"
	orchestrator_gitlab "github.com/diggerhq/digger/libs/ci/gitlab"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/xanzy/go-gitlab"
	"log"
	"os"
	"path"
)

type GitlabProvider interface {
	NewClient(token string) (*gitlab.Client, error)
}

type GitlabClientProvider struct{}

func (g GitlabClientProvider) NewClient(token string) (*gitlab.Client, error) {
	baseUrl := os.Getenv("DIGGER_GITLAB_BASE_URL")
	if baseUrl == "" {
		client, err := gitlab.NewClient(token)
		return client, err
	} else {
		client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseUrl))
		return client, err
	}
}

func GetGitlabService(gh GitlabProvider, projectId int, repoName string, repoFullName string, prNumber int, discussionId string) (*orchestrator_gitlab.GitLabService, error) {
	token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")

	client, err := gh.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("could not get gitlab client: %v", err)
	}
	context := orchestrator_gitlab.GitLabContext{
		ProjectName:      repoName,
		ProjectNamespace: repoFullName,
		ProjectId:        &projectId,
		MergeRequestIId:  &prNumber,
		DiscussionID:     discussionId,
	}
	service := orchestrator_gitlab.GitLabService{Client: client, Context: &context}
	return &service, nil
}

func GetDiggerConfigForBranch(gh GitlabProvider, projectId int, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, prNumber int, discussionId string) (string, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")

	service, err := GetGitlabService(gh, projectId, repoName, repoFullName, prNumber, discussionId)
	if err != nil {
		return "", nil, nil, fmt.Errorf("could not get gitlab service: %v", err)
	}
	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	changedFiles, err := service.GetChangedFiles(prNumber)
	if err != nil {
		log.Printf("Error getting changed files: %v", err)
		return "", nil, nil, fmt.Errorf("error getting changed files")
	}
	err = CloneGitRepoAndDoAction(cloneUrl, branch, "", token, func(dir string) error {
		diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
		diggerYmlStr = string(diggerYmlBytes)
		config, _, dependencyGraph, err = dg_configuration.LoadDiggerConfig(dir, true, changedFiles)
		if err != nil {
			log.Printf("Error loading digger config: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		log.Printf("Error cloning and loading config: %v", err)
		return "", nil, nil, fmt.Errorf("error cloning and loading config")
	}

	log.Printf("Digger config loadded successfully\n")
	return diggerYmlStr, config, dependencyGraph, nil
}
