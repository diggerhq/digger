package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	url2 "net/url"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	stscreds "github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	sts "github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/diggerhq/digger/libs/digger_config"
)

type CognitoClient interface {
	GetId(ctx context.Context, params *cognitoidentity.GetIdInput, optFns ...func(*cognitoidentity.Options)) (*cognitoidentity.GetIdOutput, error)
	GetCredentialsForIdentity(ctx context.Context, params *cognitoidentity.GetCredentialsForIdentityInput, optFns ...func(*cognitoidentity.Options)) (*cognitoidentity.GetCredentialsForIdentityOutput, error)
}

type StsClient interface {
	AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

type GithubTokenFetcher interface {
	SetAudience(audience string)
	GetIdentityToken() ([]byte, error)
}

type AuthStrategy int 
const (
	BackendConfig AuthStrategy = iota
	Terragrunt
	Cognito
)
	
/**
 * This is a singleton patterns for external data client and allows mocking for testing
 */
var defaultCognitoClient CognitoClient
var defaultStsClient StsClient
var defaultTokenFetcher GithubTokenFetcher
var defaultLoadConfig = config.LoadDefaultConfig

func getCognitoClient(cfg aws.Config) CognitoClient {
	if defaultCognitoClient == nil {
		defaultCognitoClient = cognitoidentity.NewFromConfig(cfg)
	}

	return defaultCognitoClient
}

func getTokenFeatcher() GithubTokenFetcher {
	if defaultTokenFetcher == nil {
		defaultTokenFetcher = &GithubAwsTokenFetcher{}
	}
	return defaultTokenFetcher
}

func getStsClient(cfg aws.Config) StsClient {
	if(defaultStsClient == nil) {
		defaultStsClient = sts.NewFromConfig(cfg)
	}
	return defaultStsClient
}

// Wrapper to allow for overrides if necessary
func getAwsConfig(ctx context.Context, region string) (aws.Config, error) {
	return defaultLoadConfig(ctx, config.WithRegion(region))		
}

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
	var err error 
	switch(job.GetAuthStrategy()) {
		case Cognito:
			log.Printf("Using authentication strategy: Cognito")
			err = job.AuthCognito()		
		case Terragrunt:
			log.Printf("Using authentication strategy: Terragrunt")
			err = job.AuthTerragrunt()
		case BackendConfig:
			log.Printf("Using authentication strategy: Default")
			err = job.AuthBackendConfig()			
		default:
			err = fmt.Errorf("unkown: aws authentication strategy")
	}

	if err != nil {
		return err
	}

	// If state environment variables are not set them to match command env vars 
	if len(job.StateEnvVars) == 0 && len(job.CommandEnvVars) > 0 {
		job.StateEnvVars = job.CommandEnvVars
	} 

	if len(job.StateEnvVars) > 0 && len(job.CommandEnvVars) == 0 {
		job.CommandEnvVars = job.StateEnvVars
	}  

	return nil
}

// determine the authentication strategy based on the job details
func(job *Job) GetAuthStrategy() AuthStrategy {
	if(job.Terragrunt && job.CognitoOidcConfig == nil) {
		return Terragrunt
	}
	if(job.CognitoOidcConfig != nil) {
		return Cognito
	}

	// original dfefault behavior to use backend config and aws_role_to_assume config. 
	return BackendConfig
}

// This is the default pattern that digger uses to populate the AWS credentials for the job
func(job *Job) AuthBackendConfig() error {

	if job.StateEnvProvider != nil {
		log.Printf("Project-level AWS role detected, Assuming role for project: %v", job.ProjectName)
		var err error
		backendConfigArgs, err := populateretrieveBackendConfigArgs(*job.StateEnvProvider)
		if err != nil {
			log.Printf("failed to get keys from role: %v", err)
			return fmt.Errorf("failed to get (state) keys from role: %v", err)
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
			log.Printf("failed to get keys from role: %v", err)
			return fmt.Errorf("failed to get (state) keys from role: %v", err)
		}	
	}

	if job.CommandEnvProvider != nil {
		var err error
		job.CommandEnvVars, err = populateKeys(job.CommandEnvVars, *job.CommandEnvProvider)
		if err != nil {
			log.Printf("Failed to get keys from role (CommandEnvProvider: %v", err)
			return fmt.Errorf("failed to get (command) keys from role: %v", err)
		}
	}

	return nil
}

func(job *Job) AuthTerragrunt() error {
	var err error
	if job.StateEnvProvider != nil {
		job.StateEnvVars, err = populateKeys(job.StateEnvVars, *job.StateEnvProvider)
		if err != nil {
			log.Printf("Failed to get keys from role (StateEnvProvider): %v", err)
			return fmt.Errorf("failed to get (state) keys from role: %v", err)
		}
	}

	if job.CommandEnvProvider != nil {		
		job.CommandEnvVars, err = populateKeys(job.CommandEnvVars, *job.CommandEnvProvider)
		if err != nil {
			log.Printf("Failed to get keys from role (CommandEnvProvider: %v", err)
			return fmt.Errorf("failed to get (command) keys from role: %v", err)
		}		
	}  

	return nil
}

func(job *Job) AuthCognito() error {

	log.Printf("Authenticating with Cognito for project: %v", job.ProjectName)

	creds, err := GetCognitoToken(*job.CognitoOidcConfig, "token.actions.githubusercontent.com")
	if err != nil {
		log.Printf("Failed to get Cognito token: %v", err)
		return fmt.Errorf("failed to get Cognito token: %v", err)
	}

	// If a command role provider is set and role is set then we need to chain to get those credentials
	// using the Cognito token othewise just return the Cognito token which follows the "enhanced" auth flow
	if job.CommandEnvProvider != nil || job.StateEnvProvider != nil {		

		cfg, err := getAwsConfig(context.Background(), job.CognitoOidcConfig.AwsRegion)
		if err != nil {
			log.Printf("failed to create AWS config in Auth Cognito auth strategy: %v", err)
			return fmt.Errorf("failed to create AWS config %v", err)
		}

		cfg.Credentials = credentials.NewStaticCredentialsProvider(
			creds["AWS_ACCESS_KEY_ID"],
			creds["AWS_SECRET_ACCESS_KEY"],
			creds["AWS_SESSION_TOKEN"],
		)

		stsClient := getStsClient(cfg)
		defaultStsClient = nil
		if job.StateRoleArn != "" {		

			creds, err := stsClient.AssumeRole(context.Background(), &sts.AssumeRoleInput{
				RoleArn: aws.String(job.StateRoleArn),
				RoleSessionName: aws.String(job.ProjectName + "-state"),
			})
			if err != nil {
				log.Printf("failed to assumeRole for stateRoleArn in Cognito auth strategy: %v", err)
				return fmt.Errorf("failed to assume role for state %v", err)
			}			

			job.StateEnvVars = map[string]string{
				"AWS_ACCESS_KEY_ID":  		*creds.Credentials.AccessKeyId,
				"AWS_SECRET_ACCESS_KEY": 	*creds.Credentials.SecretAccessKey,
				"AWS_SESSION_TOKEN":   		*creds.Credentials.SessionToken,
			}
		}

		if job.CommandRoleArn != "" {
			creds, err := stsClient.AssumeRole(context.Background(), &sts.AssumeRoleInput{
				RoleArn: aws.String(job.CommandRoleArn),
				RoleSessionName: aws.String(job.ProjectName + "-command"),
			}) 
			if err != nil {
				log.Printf("failed to assumeRole for commandRoleArn in Cognito auth strategy: %v", err)
				return fmt.Errorf("failed to assume role for command %v", err)
			}

			job.CommandEnvVars = map[string]string{
				"AWS_ACCESS_KEY_ID":  		*creds.Credentials.AccessKeyId,
				"AWS_SECRET_ACCESS_KEY": 	*creds.Credentials.SecretAccessKey,
				"AWS_SESSION_TOKEN":   		*creds.Credentials.SessionToken,
			}
		}

	} else {

		// Pass back the Cognito token as credentials for running commands. 
		job.StateEnvVars = creds
		job.CommandEnvVars = creds		
	}

	return nil
}

type GithubAwsTokenFetcher struct{
	audience string
}

func (fetcher *GithubAwsTokenFetcher) SetAudience(audience string) {
	fetcher.audience = audience
}

func (fetcher *GithubAwsTokenFetcher) GetIdentityToken() ([]byte, error) {
	var httpClient http.Client
	type TokenResponse struct {
		Value string `json:"value"`
	}

	audienceDomain := fetcher.audience
	if(audienceDomain == "") {
		audienceDomain = "sts.amazonaws.com"
	}

	tokenIdUrl := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	bearerToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	audience := url2.QueryEscape(audienceDomain)
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

/**
 * This gets a cognito token identity to be used for OIDC authentcation and claim mapping to Principal tags
 *  
 *  @param project config.AwsCognitoOidcConfig - the project configuration for the AWS Cognito OIDC
 *  @param idpName string - the idetity provider to use for the token i.e. token.actions.gihutusercontent.com
 *  @return map[string]string - a map of the AWS credentials to be used for the identity token from cognito
 */
func GetCognitoToken(cognitoConfig digger_config.AwsCognitoOidcConfig, idpName string) (map[string]string, error) {

	// Feature flag other idetntity providers at this point in time. 
	if idpName != "token.actions.githubusercontent.com" {
		return nil, errors.New("only github actions is supported")			
	}

	if cognitoConfig.CognitoPoolId == "" {
		return nil, errors.New("no AWS Cognito Pool Id found for project")
	}

	if cognitoConfig.AwsAccountId == "" || cognitoConfig.AwsRegion == "" {
		return nil, errors.New("account information could not be determined in order to fetch Cognito token")
	}

	cfg, err := getAwsConfig(context.Background(), cognitoConfig.AwsRegion)	
	if err != nil {
		return nil,fmt.Errorf("unable to load AWS SDK config in GetCognitoToken(), %v", err)
	}

	// We need the github access token to use as the user request an identity token from Cognito
	tokenFetcher := getTokenFeatcher()
	tokenFetcher.SetAudience("cognito-identity.amazonaws.com")
	accessToken, err := tokenFetcher.GetIdentityToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token in GetCognitoToken(): %v", err)
	}

	// Using the access token from GitHub and the Cognito pool ID, we can now request the identity token
	client := getCognitoClient(cfg)	
	getIdinput := &cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(cognitoConfig.CognitoPoolId),
		Logins: map[string]string{
			"token.actions.githubusercontent.com": string(accessToken),
		},
		AccountId: aws.String(cognitoConfig.AwsAccountId),
	}

	getIdOutput, err := client.GetId(context.Background(), getIdinput)
	if err != nil {
		return nil, fmt.Errorf("failed to get a valid cognito id token: %v", err)
	}

	// Now that we have the identity ID, we can get the credentials for the identity
	getCredInput := &cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: aws.String(*getIdOutput.IdentityId),
		Logins: map[string]string{
			"token.actions.githubusercontent.com": string(accessToken),
		},
	}

	getCredsOutput, err := client.GetCredentialsForIdentity(context.Background(), getCredInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get a valid cognito id token: %v", err)
	}

	/**
	 * @TODO replace this with a struct type for these credentials, for now return 
	 * a map similar to the one in populateKeys() method
	 */
	return map[string]string{
		"AWS_ACCESS_KEY_ID":  		*getCredsOutput.Credentials.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY": 	*getCredsOutput.Credentials.SecretKey,
		"AWS_SESSION_TOKEN":   		*getCredsOutput.Credentials.SessionToken,
	}, nil

}

// pool ids are in this format: <region>:<guid>
func parseRegionFromPoolId(poolId string) string {
	return poolId[:9]
}
