package utils

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/dominikbraun/graph"
	orchestrator_bitbucket "github.com/go-substrate/strate/libs/ci/bitbucket"
	dg_configuration "github.com/go-substrate/strate/libs/digger_config"
	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketProvider interface {
	NewClient(token string) (*bitbucket.Client, error)
}

type BitbucketClientProvider struct{}

func (b BitbucketClientProvider) NewClient(token string) (*bitbucket.Client, error) {
	client := bitbucket.NewOAuthbearerToken(token)
	return client, nil
}

func GetBitbucketService(bb BitbucketProvider, token string, repoOwner string, repoName string, prNumber int) (*orchestrator_bitbucket.BitbucketAPI, error) {
	// token := os.Getenv("DIGGER_BITBUCKET_ACCESS_TOKEN")

	//client, err := bb.NewClient(token)
	//if err != nil {
	//	return nil, fmt.Errorf("could not get bitbucket client: %v", err)
	//}
	//context := orchestrator_bitbucket.BitbucketContext{
	//	RepositoryName:     repoName,
	//	RepositoryFullName: repoFullName,
	//	PullRequestID:      &prNumber,
	//}
	service := orchestrator_bitbucket.BitbucketAPI{
		AuthToken:     token,
		RepoWorkspace: repoOwner,
		RepoName:      repoName,
		HttpClient:    http.Client{},
	}
	return &service, nil
}

func GetDiggerConfigForBitbucketBranch(bb BitbucketProvider, token string, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, prNumber int) (string, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	service, err := GetBitbucketService(bb, token, repoOwner, repoName, prNumber)
	if err != nil {
		return "", nil, nil, fmt.Errorf("could not get bitbucket service: %v", err)
	}
	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	changedFiles, err := service.GetChangedFiles(prNumber)
	if err != nil {
		log.Printf("Error getting changed files: %v", err)
		return "", nil, nil, fmt.Errorf("error getting changed files")
	}

	err = CloneGitRepoAndDoAction(cloneUrl, branch, "", token, "x-token-auth", func(dir string) error {
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

	log.Printf("Digger config loaded successfully\n")
	return diggerYmlStr, config, dependencyGraph, nil
}
