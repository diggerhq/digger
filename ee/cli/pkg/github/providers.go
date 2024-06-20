package github

import (
	"fmt"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/google/go-github/v61/github"
	"log"
	"os"
)

type GithubServiceProviderAdvanced struct{}

func (_ GithubServiceProviderAdvanced) NewService(ghToken string, repoName string, owner string) (dg_github.GithubService, error) {
	client := github.NewClient(nil)
	if ghToken != "" {
		client = client.WithAuthToken(ghToken)
	}

	githubHostname := os.Getenv("DIGGER_GITHUB_HOSTNAME")
	var err error
	if githubHostname != "" {
		log.Printf("info: using github hostname: %v", githubHostname)
		githubEnterpriseBaseUrl := fmt.Sprintf("https://%v/api/v3/", githubHostname)
		githubEnterpriseUploadUrl := fmt.Sprintf("https://%v/api/uploads/", githubHostname)
		client, err = client.WithEnterpriseURLs(githubEnterpriseBaseUrl, githubEnterpriseUploadUrl)
		if err != nil {
			log.Printf("error: could not create enterprise client: %v", err)
			return dg_github.GithubService{}, fmt.Errorf("could not create enterprise client: %v", err)
		}
	}

	return dg_github.GithubService{
		Client:   client,
		RepoName: repoName,
		Owner:    owner,
	}, nil
}
