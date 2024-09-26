package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/next/utils"
	"github.com/google/go-github/v61/github"
	"log"
	net "net/http"
	"os"
	"strconv"
)

func GetGithubClient(gh utils.GithubClientProvider, installationId int64, repoFullName string) (*github.Client, *string, error) {
	repo, err := dbmodels.DB.GetRepoByInstllationIdAndRepoFullName(strconv.FormatInt(installationId, 10), repoFullName)
	if err != nil {
		log.Printf("Error getting repo: %v", err)
		return nil, nil, fmt.Errorf("Error getting repo: %v", err)
	}

	ghClient, token, err := gh.Get(repo.GithubAppID, installationId)
	return ghClient, token, err
}

func GetGithubService(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string) (*github2.GithubService, *string, error) {
	ghClient, token, err := GetGithubClient(gh, installationId, repoFullName)
	if err != nil {
		log.Printf("Error creating github app client: %v", err)
		return nil, nil, fmt.Errorf("Error creating github app client: %v", err)
	}

	ghService := github2.GithubService{
		Client:   ghClient,
		RepoName: repoName,
		Owner:    repoOwner,
	}

	return &ghService, token, nil
}

type DiggerGithubRealClientProvider struct {
}

func (gh DiggerGithubRealClientProvider) NewClient(netClient *net.Client) (*github.Client, error) {
	ghClient := github.NewClient(netClient)
	return ghClient, nil
}

func (gh DiggerGithubRealClientProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
	githubAppPrivateKey := ""
	githubAppPrivateKeyB64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")
	if githubAppPrivateKeyB64 != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(githubAppPrivateKeyB64)
		if err != nil {
			return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
		}
		githubAppPrivateKey = string(decodedBytes)
	} else {
		githubAppPrivateKey = os.Getenv("GITHUB_APP_PRIVATE_KEY")
		if githubAppPrivateKey != "" {
			log.Printf("WARNING: GITHUB_APP_PRIVATE_KEY will be deprecated in future releases, " +
				"please use GITHUB_APP_PRIVATE_KEY_BASE64 instead")
		} else {
			return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
		}
	}

	tr := net.DefaultTransport
	itr, err := ghinstallation.New(tr, githubAppId, installationId, []byte(githubAppPrivateKey))
	if err != nil {
		return nil, nil, fmt.Errorf("error initialising github app installation: %v\n", err)
	}

	token, err := itr.Token(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("error initialising git app token: %v\n", err)
	}
	ghClient, err := gh.NewClient(&net.Client{Transport: itr})
	if err != nil {
		log.Printf("error creating new client: %v", err)
	}
	return ghClient, &token, nil
}
