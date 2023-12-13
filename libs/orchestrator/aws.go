package orchestrator

import (
	"encoding/json"
	"fmt"
	awssdkcreds "github.com/aws/aws-sdk-go/aws/credentials"
	stscreds "github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	sts "github.com/aws/aws-sdk-go/service/sts"
	"log"
	"net/http"
	"os"
)

func populateKeys(envs map[string]string, provider stscreds.WebIdentityRoleProvider) (map[string]string, error) {
	creds, err := provider.Retrieve()
	if err != nil {
		return envs, fmt.Errorf("populateKeys: Could not retrieve keys from provider %v", err)
	}
	envs["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
	envs["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
	envs["AWS_SESSION_TOKEN"] = creds.SessionToken
	return envs, nil
}

func (job *Job) PopulateAwsCredentialsEnvVarsForJob() error {

	if job.StateEnvProvider != nil {
		log.Printf("Project-level AWS role detected, Assuming role: %v for project run: %v", job.ProjectName)
		var err error
		job.StateEnvVars, err = populateKeys(job.StateEnvVars, *job.StateEnvProvider)
		if err != nil {
			log.Printf("Failed to get keys from role: %v", err)
			return fmt.Errorf("Failed to get (state) keys from role: %v", err)
		}
	}

	if job.CommandEnvProvider != nil {
		var err error
		job.CommandEnvVars, err = populateKeys(job.CommandEnvVars, *job.CommandEnvProvider)
		if err != nil {
			log.Printf("Failed to get keys from role (CommandEnvProvider: %v", err)
			return fmt.Errorf("Failed to get (command) keys from role: %v", err)
		}
	}
	return nil
}

type GithubAwsTokenFetcher struct{}

func (fetcher GithubAwsTokenFetcher) FetchToken(context awssdkcreds.Context) ([]byte, error) {
	var httpClient http.Client
	type TokenResponse struct {
		Value []byte
	}
	tokenIdUrl := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	audience := "sts.amazonaws.com"
	url := fmt.Sprintf("%v&audience=%v", tokenIdUrl, audience)
	req, err := http.NewRequest("GET", url, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	parsed := &TokenResponse{}
	json.NewDecoder(resp.Body).Decode(parsed)
	return parsed.Value, nil
}

func GetProviderFromRole(role string) *stscreds.WebIdentityRoleProvider {
	mySession := session.Must(session.NewSession())
	stsSTS := sts.New(mySession)
	x := stscreds.NewWebIdentityRoleProviderWithOptions(stsSTS, role, role, GithubAwsTokenFetcher{})
	return x
}
