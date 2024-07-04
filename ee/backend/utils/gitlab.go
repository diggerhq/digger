package utils

import (
	"fmt"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/dominikbraun/graph"
	"github.com/xanzy/go-gitlab"
	"os"
	"path"
)

type GitlabProvider interface {
	NewClient(token string) (*gitlab.Client, error)
}

type GitlabClientProvider struct{}

func (g GitlabClientProvider) NewClient(token string) (*gitlab.Client, error) {
	client, err := gitlab.NewClient(token)
	return client, err
}

func getDiggerConfigForBranch(gh GitlabProvider, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
	client, err := gh.NewClient(token)
	if err != nil {
		return fmt.Errorf("")
	}

	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	changedFiles, err := ghService.GetChangedFiles(prNumber)
	if err != nil {
		log.Printf("Error getting changed files: %v", err)
		return "", nil, nil, nil, fmt.Errorf("error getting changed files")
	}
	err = utils.CloneGitRepoAndDoAction(cloneUrl, branch, *token, func(dir string) error {
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
		return "", nil, nil, nil, fmt.Errorf("error cloning and loading config")
	}

	log.Printf("Digger config loadded successfully\n")
	return diggerYmlStr, ghService, config, dependencyGraph, nil
}
