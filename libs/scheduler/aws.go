package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	url2 "net/url"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	stscreds "github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	sts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/diggerhq/digger/libs/digger_config"
)

func populateretrieveBackendConfigArgs(provider stscreds.WebIdentityRoleProvider) ([]string, error) {
	creds, err := provider.Retrieve(context.Background())
	var args []string
	if err != nil {
		return args, fmt.Errorf("populateKeys: Could not retrieve keys from provider %v", err)
	}
	accessKey := fmt.Sprintf("-backend-config=access_key=%v", creds.AccessKeyID)
	secretKey := fmt.Sprintf("-backend-config=secret_key=%v", creds.SecretAccessKey)
	token := fmt.Sprintf("-backend-config=token=%v", creds.SessionToken)
	return append(args, accessKey, secretKey, token), nil

}

func populateKeys(envs map[string]string, provider stscreds.WebIdentityRoleProvider) (map[string]string, error) {
	creds, err := provider.Retrieve(context.Background())
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
		log.Printf("Project-level AWS role detected, Assuming role for project: %v", job.ProjectName)
		var err error
		backendConfigArgs, err := populateretrieveBackendConfigArgs(*job.StateEnvProvider)

		// Terragrunt will cause a backend configuration problem if backend-config options are passed and envs of the same key are passed. 
		// which will trigger a request to init with --reconfigure, so do not use backend-config for terragrunt	
		if job.Terragrunt != true {
		if err != nil {
			log.Printf("Failed to get keys from role: %v", err)
			return fmt.Errorf("Failed to get (state) keys from role: %v", err)
		}

		if job.PlanStage != nil {
			// TODO: check that the first step is infact the terraform "init" step
			job.PlanStage.Steps[0].ExtraArgs = append(job.PlanStage.Steps[0].ExtraArgs, backendConfigArgs...)
		}
		if job.ApplyStage != nil {
			// TODO: check that the first step is infact the terraform "init" step
			job.ApplyStage.Steps[0].ExtraArgs = append(job.ApplyStage.Steps[0].ExtraArgs, backendConfigArgs...)
		}
		if err != nil {
			log.Printf("Failed to get keys from role: %v", err)
			return fmt.Errorf("Failed to get (state) keys from role: %v", err)
		}
		} else {
			job.StateEnvVars, err = populateKeys(job.StateEnvVars, *job.StateEnvProvider)
			if err != nil {
				log.Printf("Failed to get keys from role (StateEnvProvider): %v", err)
				return fmt.Errorf("Failed to get (state) keys from role: %v", err)
			}
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

	// If state environment variables are not set them to match command env vars 
	if len(job.StateEnvVars) == 0 && len(job.CommandEnvVars) != 0 {
		job.StateEnvVars = job.CommandEnvVars
	} 

	if len(job.StateEnvVars) != 0 && len(job.CommandEnvVars) == 0 {
		job.CommandEnvVars = job.StateEnvVars
	}  

	return nil
}

type GithubAwsTokenFetcher struct{}

func (fetcher GithubAwsTokenFetcher) GetIdentityToken() ([]byte, error) {
	var httpClient http.Client
	type TokenResponse struct {
		Value string `json:"value"`
	}
	tokenIdUrl := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	bearerToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	audience := url2.QueryEscape("sts.amazonaws.com")
	url := fmt.Sprintf("%v&audience=%v", tokenIdUrl, audience)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("bearer  %v", bearerToken))
	req.Header.Add("Accept", "application/json; api-version=2.0")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "actions/oidc-client")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	parsed := &TokenResponse{}
	json.NewDecoder(resp.Body).Decode(parsed)
	return []byte(parsed.Value), nil
}

func GetProviderFromRole(role string, region string) *stscreds.WebIdentityRoleProvider {
	if role == "" {
		return nil
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Printf("Failed to create aws session: %v", err)
		return nil
	}
	stsClient := sts.NewFromConfig(cfg)
	x := stscreds.NewWebIdentityRoleProvider(stsClient, role, &GithubAwsTokenFetcher{})
	return x
}

func GetStateAndCommandProviders(project digger_config.Project) (*stscreds.WebIdentityRoleProvider, *stscreds.WebIdentityRoleProvider) {
	var StateEnvProvider *stscreds.WebIdentityRoleProvider
	var CommandEnvProvider *stscreds.WebIdentityRoleProvider
	if project.AwsRoleToAssume != nil {

		if project.AwsRoleToAssume.State != "" {
			StateEnvProvider = GetProviderFromRole(project.AwsRoleToAssume.State, project.AwsRoleToAssume.AwsRoleRegion)
		} else {
			StateEnvProvider = nil
		}

		if project.AwsRoleToAssume.Command != "" {
			CommandEnvProvider = GetProviderFromRole(project.AwsRoleToAssume.Command, project.AwsRoleToAssume.AwsRoleRegion)
		} else {
			CommandEnvProvider = nil
		}
	}
	return StateEnvProvider, CommandEnvProvider
}
