package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
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
	if githubHostname != "" {
		githubEnterpriseBaseUrl, githubEnterpriseUploadUrl := getGithubEnterpriseUrls(githubHostname)
		log.Printf("Info: Using digger enterprise instance: base url: %v", githubEnterpriseBaseUrl)
		ghClient, err = ghClient.WithEnterpriseURLs(githubEnterpriseBaseUrl, githubEnterpriseUploadUrl)
		if err != nil {
			return nil, fmt.Errorf("could not create github enterprise client: %v", err)
		}
	}
	return ghClient, nil
}

func getGithubEnterpriseUrls(githubHostname string) (string, string) {
	githubEnterpriseBaseUrl := fmt.Sprintf("https://%v/api/v3/", githubHostname)
	githubEnterpriseUploadUrl := fmt.Sprintf("https://%v/api/uploads/", githubHostname)
	return githubEnterpriseBaseUrl, githubEnterpriseUploadUrl
}

func (gh DiggerGithubEEClientProvider) Get(githubAppId int64, installationId int64) (*github.Client, *string, error) {
	githubAppPrivateKey := ""
	githubAppPrivateKeyB64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")

	if githubAppPrivateKeyB64 == "" {
		log.Printf("Info: could not find env variable for private key, attempting to find via connection ID")
		connectionEnc, err := models.DB.GetGithubAppConnection(githubAppId)
		if err != nil {
			log.Printf("could not find app using app id: %v", githubAppId)
			return nil, nil, fmt.Errorf("could not find app using app id: %v", githubAppId)
		}

		secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
		if secret == "" {
			log.Printf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
			return nil, nil, fmt.Errorf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
		}
		connection, err := utils.DecryptConnection(connectionEnc, []byte(secret))
		if err != nil {
			log.Printf("could not decrypt connection value: %v", err)
			return nil, nil, fmt.Errorf("could not decrypt connection value")
		}

		githubAppPrivateKeyB64 = connection.PrivateKeyBase64
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(githubAppPrivateKeyB64)
	if err != nil {
		return nil, nil, fmt.Errorf("error initialising github app installation: please set GITHUB_APP_PRIVATE_KEY_BASE64 env variable\n")
	}
	githubAppPrivateKey = string(decodedBytes)

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
	githubHostname := os.Getenv("DIGGER_GITHUB_HOSTNAME")
	if githubHostname != "" {
		githubEnterpriseBaseUrl, _ := getGithubEnterpriseUrls(githubHostname)
		itr.BaseURL = githubEnterpriseBaseUrl
	}
	token, err := itr.Token(context.Background())

	return ghClient, &token, nil
}

func (gh DiggerGithubEEClientProvider) FetchCredentials(githubAppId string) (string, string, string, string, error) {
	clientId := os.Getenv("GITHUB_APP_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_APP_CLIENT_SECRET")
	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	privateKeyb64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")

	if clientId != "" && clientSecret != "" && webhookSecret != "" && privateKeyb64 != "" {
		log.Printf("Info: found github client credentials from env variables, using those")
		return clientId, clientSecret, webhookSecret, privateKeyb64, nil
	}

	log.Printf("Info: client ID and secret env variables not set, trying to find app via connection ID")

	connectionEnc, err := models.DB.GetGithubAppConnection(githubAppId)
	if err != nil {
		log.Printf("could not find app using githubAppId id: %v", githubAppId)
		return "", "", "", "", fmt.Errorf("could not find app using githubAppId id: %v", githubAppId)
	}

	secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
	if secret == "" {
		log.Printf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
		return "", "", "", "", fmt.Errorf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
	}
	connection, err := utils.DecryptConnection(connectionEnc, []byte(secret))
	if err != nil {
		log.Printf("could not decrypt connection value: %v", err)
		return "", "", "", "", fmt.Errorf("could not decrypt connection value")
	}
	clientId = connection.ClientID
	clientSecret = connection.ClientSecret
	webhookSecret = connection.WebhookSecret
	privateKeyb64 = connection.PrivateKeyBase64

	return clientId, clientSecret, webhookSecret, privateKeyb64, nil
}
