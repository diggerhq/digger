package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
var (
	defaultCognitoClient CognitoClient
	defaultStsClient     StsClient
	defaultTokenFetcher  GithubTokenFetcher
	defaultLoadConfig    = config.LoadDefaultConfig
)

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
	if defaultStsClient == nil {
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
		slog.Error("Could not retrieve keys from provider", "error", err)
		return args, fmt.Errorf("populateKeys: Could not retrieve keys from provider %v", err)
	}
	accessKey := fmt.Sprintf("-backend-config=access_key=%v", creds.AccessKeyID)
	secretKey := fmt.Sprintf("-backend-config=secret_key=%v", creds.SecretAccessKey)
	token := fmt.Sprintf("-backend-config=token=%v", creds.SessionToken)

	slog.Debug("Retrieved backend config arguments successfully")
	return append(args, accessKey, secretKey, token), nil
}

func populateKeys(envs map[string]string, provider stscreds.WebIdentityRoleProvider) (map[string]string, error) {
	creds, err := provider.Retrieve(context.Background())
	if err != nil {
		slog.Error("Could not retrieve keys from provider", "error", err)
		return envs, fmt.Errorf("populateKeys: Could not retrieve keys from provider %v", err)
	}
	envs["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
	envs["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
	envs["AWS_SESSION_TOKEN"] = creds.SessionToken

	slog.Debug("Populated environment variables with AWS credentials")
	return envs, nil
}

func (job *Job) PopulateAwsCredentialsEnvVarsForJob() error {
	var err error
	switch job.GetAuthStrategy() {
	case Cognito:
		slog.Info("Using authentication strategy: Cognito")
		err = job.AuthCognito()
	case Terragrunt:
		slog.Info("Using authentication strategy: Terragrunt")
		err = job.AuthTerragrunt()
	case BackendConfig:
		slog.Info("Using authentication strategy: Default")
		err = job.AuthBackendConfig()
	default:
		err = fmt.Errorf("unknown: aws authentication strategy")
	}

	if err != nil {
		return err
	}

	// If state environment variables are not set them to match command env vars
	if len(job.StateEnvVars) == 0 && len(job.CommandEnvVars) > 0 {
		slog.Debug("Copying command environment variables to state environment variables")
		job.StateEnvVars = job.CommandEnvVars
	}

	if len(job.StateEnvVars) > 0 && len(job.CommandEnvVars) == 0 {
		slog.Debug("Copying state environment variables to command environment variables")
		job.CommandEnvVars = job.StateEnvVars
	}

	return nil
}

// determine the authentication strategy based on the job details
func (job *Job) GetAuthStrategy() AuthStrategy {
	if job.Terragrunt && job.CognitoOidcConfig == nil {
		return Terragrunt
	}
	if job.CognitoOidcConfig != nil {
		return Cognito
	}

	// original default behavior to use backend config and aws_role_to_assume config.
	return BackendConfig
}

// This is the default pattern that digger uses to populate the AWS credentials for the job
func (job *Job) AuthBackendConfig() error {
	if job.StateEnvProvider != nil {
		slog.Info("Project-level AWS role detected, assuming role for project", "project", job.ProjectName)

		var err error
		backendConfigArgs, err := populateretrieveBackendConfigArgs(*job.StateEnvProvider)
		if err != nil {
			slog.Error("Failed to get keys from role for state", "error", err)
			return fmt.Errorf("failed to get (state) keys from role: %v", err)
		}

		if job.PlanStage != nil {
			// TODO: check that the first step is in fact the terraform "init" step
			slog.Debug("Adding backend config arguments to plan stage")
			job.PlanStage.Steps[0].ExtraArgs = append(job.PlanStage.Steps[0].ExtraArgs, backendConfigArgs...)
		}
		if job.ApplyStage != nil {
			// TODO: check that the first step is in fact the terraform "init" step
			slog.Debug("Adding backend config arguments to apply stage")
			job.ApplyStage.Steps[0].ExtraArgs = append(job.ApplyStage.Steps[0].ExtraArgs, backendConfigArgs...)
		}
	}

	if job.CommandEnvProvider != nil {
		slog.Debug("Setting command environment variables from provider")
		var err error
		job.CommandEnvVars, err = populateKeys(job.CommandEnvVars, *job.CommandEnvProvider)
		if err != nil {
			slog.Error("Failed to get keys from role for command", "error", err)
			return fmt.Errorf("failed to get (command) keys from role: %v", err)
		}
	}

	return nil
}

func (job *Job) AuthTerragrunt() error {
	var err error
	if job.StateEnvProvider != nil {
		slog.Debug("Setting state environment variables from provider")
		job.StateEnvVars, err = populateKeys(job.StateEnvVars, *job.StateEnvProvider)
		if err != nil {
			slog.Error("Failed to get keys from role for state", "error", err)
			return fmt.Errorf("failed to get (state) keys from role: %v", err)
		}
	}

	if job.CommandEnvProvider != nil {
		slog.Debug("Setting command environment variables from provider")
		job.CommandEnvVars, err = populateKeys(job.CommandEnvVars, *job.CommandEnvProvider)
		if err != nil {
			slog.Error("Failed to get keys from role for command", "error", err)
			return fmt.Errorf("failed to get (command) keys from role: %v", err)
		}
	}

	return nil
}

func (job *Job) AuthCognito() error {
	slog.Info("Authenticating with Cognito for project", "project", job.ProjectName)

	creds, err := GetCognitoToken(*job.CognitoOidcConfig, "token.actions.githubusercontent.com")
	if err != nil {
		slog.Error("Failed to get Cognito token", "error", err)
		return fmt.Errorf("failed to get Cognito token: %v", err)
	}

	// If a command role provider is set and role is set then we need to chain to get those credentials
	// using the Cognito token otherwise just return the Cognito token which follows the "enhanced" auth flow
	if job.CommandEnvProvider != nil || job.StateEnvProvider != nil {
		slog.Debug("Command or state role provider set, chaining credentials")

		cfg, err := getAwsConfig(context.Background(), job.CognitoOidcConfig.AwsRegion)
		if err != nil {
			slog.Error("Failed to create AWS config in Cognito auth strategy", "error", err)
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
			slog.Debug("Assuming state role", "roleArn", job.StateRoleArn)

			assumeRoleResult, err := stsClient.AssumeRole(context.Background(), &sts.AssumeRoleInput{
				RoleArn:         aws.String(job.StateRoleArn),
				RoleSessionName: aws.String(job.ProjectName + "-state"),
			})
			if err != nil {
				slog.Error("Failed to assume role for state", "roleArn", job.StateRoleArn, "error", err)
				return fmt.Errorf("failed to assume role for state %v", err)
			}

			job.StateEnvVars = map[string]string{
				"AWS_ACCESS_KEY_ID":     *assumeRoleResult.Credentials.AccessKeyId,
				"AWS_SECRET_ACCESS_KEY": *assumeRoleResult.Credentials.SecretAccessKey,
				"AWS_SESSION_TOKEN":     *assumeRoleResult.Credentials.SessionToken,
			}

			slog.Debug("Successfully assumed state role")
		}

		if job.CommandRoleArn != "" {
			slog.Debug("Assuming command role", "roleArn", job.CommandRoleArn)

			assumeRoleResult, err := stsClient.AssumeRole(context.Background(), &sts.AssumeRoleInput{
				RoleArn:         aws.String(job.CommandRoleArn),
				RoleSessionName: aws.String(job.ProjectName + "-command"),
			})
			if err != nil {
				slog.Error("Failed to assume role for command", "roleArn", job.CommandRoleArn, "error", err)
				return fmt.Errorf("failed to assume role for command %v", err)
			}

			job.CommandEnvVars = map[string]string{
				"AWS_ACCESS_KEY_ID":     *assumeRoleResult.Credentials.AccessKeyId,
				"AWS_SECRET_ACCESS_KEY": *assumeRoleResult.Credentials.SecretAccessKey,
				"AWS_SESSION_TOKEN":     *assumeRoleResult.Credentials.SessionToken,
			}

			slog.Debug("Successfully assumed command role")
		}
	} else {
		slog.Debug("Using Cognito token directly for credentials")
		// Pass back the Cognito token as credentials for running commands.
		job.StateEnvVars = creds
		job.CommandEnvVars = creds
	}

	return nil
}

type GithubAwsTokenFetcher struct {
	audience string
}

func (fetcher *GithubAwsTokenFetcher) SetAudience(audience string) {
	slog.Debug("GithubAwsTokenFetcher setting audience", "audience", audience)
	fetcher.audience = audience
}

func (fetcher *GithubAwsTokenFetcher) GetIdentityToken() ([]byte, error) {
	var httpClient http.Client
	type TokenResponse struct {
		Value string `json:"value"`
	}

	audienceDomain := fetcher.audience
	if audienceDomain == "" {
		audienceDomain = "sts.amazonaws.com"
	}

	tokenIdUrl := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	bearerToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	audience := url2.QueryEscape(audienceDomain)
	url := fmt.Sprintf("%v&audience=%v", tokenIdUrl, audience)

	slog.Debug("Fetching GitHub identity token", "audience", audienceDomain)
	slog.Debug("len(ACTIONS_ID_TOKEN_REQUEST_URL)", "length of ACTIONS_ID_TOKEN_REQUEST_URL", len(tokenIdUrl))
	slog.Debug("len(ACTIONS_ID_TOKEN_REQUEST_TOKEN)", "length of ACTIONS_ID_TOKEN_REQUEST_TOKEN", len(bearerToken))
	slog.Debug("audience (escaped", "audience", audience)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Failed to create request for GitHub identity token", "error", err)
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("bearer  %v", bearerToken))
	req.Header.Add("Accept", "application/json; api-version=2.0")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "actions/oidc-client")

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to fetch GitHub identity token", "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	parsed := &TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(parsed); err != nil {
		slog.Error("Failed to decode token response", "error", err)
		return nil, fmt.Errorf("failed to decode token response: %v", err)
	}

	slog.Debug("Successfully fetched GitHub identity token")
	return []byte(parsed.Value), nil
}

func GetProviderFromRole(role, region string) *stscreds.WebIdentityRoleProvider {
	if role == "" {
		return nil
	}

	slog.Debug("Creating web identity role provider", "role", role, "region", region)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		slog.Error("Failed to create AWS session", "error", err)
		return nil
	}
	stsClient := sts.NewFromConfig(cfg)
	provider := stscreds.NewWebIdentityRoleProvider(stsClient, role, &GithubAwsTokenFetcher{})
	return provider
}

func GetStateAndCommandProviders(project digger_config.Project) (*stscreds.WebIdentityRoleProvider, *stscreds.WebIdentityRoleProvider) {
	var StateEnvProvider *stscreds.WebIdentityRoleProvider
	var CommandEnvProvider *stscreds.WebIdentityRoleProvider

	if project.AwsRoleToAssume != nil {
		slog.Debug("Setting up AWS role providers",
			"stateRole", project.AwsRoleToAssume.State,
			"commandRole", project.AwsRoleToAssume.Command,
			"region", project.AwsRoleToAssume.AwsRoleRegion)

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
 * This gets a cognito token identity to be used for OIDC authentication and claim mapping to Principal tags
 *
 *  @param project config.AwsCognitoOidcConfig - the project configuration for the AWS Cognito OIDC
 *  @param idpName string - the identity provider to use for the token i.e. token.actions.githubusercontent.com
 *  @return map[string]string - a map of the AWS credentials to be used for the identity token from cognito
 */
func GetCognitoToken(cognitoConfig digger_config.AwsCognitoOidcConfig, idpName string) (map[string]string, error) {
	slog.Debug("Getting Cognito token",
		"idpName", idpName,
		"poolId", cognitoConfig.CognitoPoolId)

	// Feature flag other identity providers at this point in time.
	if idpName != "token.actions.githubusercontent.com" {
		slog.Error("Unsupported identity provider", "idpName", idpName)
		return nil, errors.New("only github actions is supported")
	}

	if cognitoConfig.CognitoPoolId == "" {
		slog.Error("No AWS Cognito Pool ID found for project")
		return nil, errors.New("no AWS Cognito Pool Id found for project")
	}

	if cognitoConfig.AwsAccountId == "" || cognitoConfig.AwsRegion == "" {
		slog.Error("Missing account information for Cognito token",
			"accountId", cognitoConfig.AwsAccountId,
			"region", cognitoConfig.AwsRegion)
		return nil, errors.New("account information could not be determined in order to fetch Cognito token")
	}

	cfg, err := getAwsConfig(context.Background(), cognitoConfig.AwsRegion)
	if err != nil {
		slog.Error("Unable to load AWS SDK config", "error", err)
		return nil, fmt.Errorf("unable to load AWS SDK config in GetCognitoToken(), %v", err)
	}

	// We need the github access token to use as the user request an identity token from Cognito
	tokenFetcher := getTokenFeatcher()
	tokenFetcher.SetAudience("cognito-identity.amazonaws.com")

	slog.Debug("Fetching GitHub identity token for Cognito")
	accessToken, err := tokenFetcher.GetIdentityToken()
	if err != nil {
		slog.Error("Failed to get access token for Cognito", "error", err)
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

	slog.Debug("Getting Cognito identity ID")
	getIdOutput, err := client.GetId(context.Background(), getIdinput)
	if err != nil {
		slog.Error("Failed to get a valid Cognito ID token", "error", err)
		return nil, fmt.Errorf("failed to get a valid cognito id token: %v", err)
	}

	// Now that we have the identity ID, we can get the credentials for the identity
	getCredInput := &cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: aws.String(*getIdOutput.IdentityId),
		Logins: map[string]string{
			"token.actions.githubusercontent.com": string(accessToken),
		},
	}

	slog.Debug("Getting Cognito credentials for identity", "identityId", *getIdOutput.IdentityId)
	getCredsOutput, err := client.GetCredentialsForIdentity(context.Background(), getCredInput)
	if err != nil {
		slog.Error("Failed to get credentials for Cognito identity", "error", err)
		return nil, fmt.Errorf("failed to get a valid cognito id token: %v", err)
	}

	slog.Info("Successfully obtained Cognito credentials")
	/**
	 * @TODO replace this with a struct type for these credentials, for now return
	 * a map similar to the one in populateKeys() method
	 */
	return map[string]string{
		"AWS_ACCESS_KEY_ID":     *getCredsOutput.Credentials.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY": *getCredsOutput.Credentials.SecretKey,
		"AWS_SESSION_TOKEN":     *getCredsOutput.Credentials.SessionToken,
	}, nil
}

// pool ids are in this format: <region>:<guid>
func parseRegionFromPoolId(poolId string) string {
	return poolId[:9]
}
