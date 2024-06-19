package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v61/github"
	"log"
	net "net/http"
	"os"
)

type DiggerGithubEEClientProvider struct {
}

func (gh DiggerGithubEEClientProvider) NewClient(netClient *net.Client) (*github.Client, error) {
	ghClient := github.NewClient(netClient)
	var err error
	// checking for enterprise state
	githubHostname := os.Getenv("DIGGER_GITHUB_HOSTNAME")
	if githubHostname == "1" {
		githubEnterpriseBaseUrl := fmt.Sprintf("https://%v/api/v3/", githubHostname)
		githubEnterpriseUploadUrl := fmt.Sprintf("https://%v/api/uploads/", githubHostname)
		log.Printf("Info: Using digger enterprise instance: base url: %v", githubEnterpriseBaseUrl)
		ghClient, err = ghClient.WithEnterpriseURLs(githubEnterpriseBaseUrl, githubEnterpriseUploadUrl)
		if err != nil {
			return nil, fmt.Errorf("could not create github enterprise client: %v", err)
		}
	}
	return ghClient, nil
}

func (gh DiggerGithubEEClientProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
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

	if err != nil {
		return nil, nil, fmt.Errorf("error initialising git app token: %v\n", err)
	}

	ghClient, err := gh.NewClient(&net.Client{Transport: itr})
	if err != nil {
		return nil, nil, fmt.Errorf("could not get digger client: %v", err)
	}
	// checking for enterprise state
	useGithubEnterprise := os.Getenv("DIGGER_USE_GITHUB_ENTERPRISE")
	if useGithubEnterprise != "" {
		githubEnterpriseBaseUrl := os.Getenv("DIGGER_GITHUB_ENTERPRISE_BASE_URL")
		itr.BaseURL = githubEnterpriseBaseUrl
	}
	token, err := itr.Token(context.Background())

	return ghClient, &token, nil
}
