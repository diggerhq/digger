package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
)

type AwsRoleProviderMock struct {
	AwsKey          string
	AwsSecret       string
	AwsSessionToken string
	AwsProviderName string
}

// Mock client for testing cognito token retrieval
type MockCognitoClient struct{}
func (m *MockCognitoClient) GetId(ctx context.Context, params *cognitoidentity.GetIdInput, optFns ...func(*cognitoidentity.Options)) (*cognitoidentity.GetIdOutput, error) {
	if params.IdentityPoolId == nil || *params.IdentityPoolId == "" {
		return nil, errors.New("IdentityPoolId is required")
	}
	// Return a mock response
	return &cognitoidentity.GetIdOutput{
		IdentityId: aws.String("mock-identity-id"),
	}, nil
}
func (m *MockCognitoClient) GetCredentialsForIdentity(ctx context.Context, params *cognitoidentity.GetCredentialsForIdentityInput, optFns ...func(*cognitoidentity.Options)) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	if params.IdentityId == nil || *params.IdentityId == "" {
		return nil, errors.New("IdentityId is required")
	}
	// Return a mock response
	return &cognitoidentity.GetCredentialsForIdentityOutput{
		Credentials: &types.Credentials{
			AccessKeyId:  aws.String("mock-access-key-id"),
			SecretKey:    aws.String("mock-secret-access-key"),
			SessionToken: aws.String("mock-session-token"),
		},
	}, nil
}

type MockTokenFetcher struct {}
func (m *MockTokenFetcher) SetAudience(audience string) {} 
func (m *MockTokenFetcher) GetIdentityToken() ([]byte, error) {
	return []byte("mock-identity-token"), nil
}


func setupCognitoMocks() (*MockCognitoClient, *MockTokenFetcher) {
	client := MockCognitoClient{}
	tokenFetcher := MockTokenFetcher{}
	defaultCognitoClient = &client
	defaultTokenFetcher = &tokenFetcher
	defaultLoadConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) (error)) (cfg aws.Config, err error) {
		return aws.Config{}, nil   // Mock the aws.Config struct
	}
	return &client, &tokenFetcher
}

func (a *AwsRoleProviderMock) Retrieve() (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     a.AwsKey,
		SecretAccessKey: a.AwsSecret,
		SessionToken:    a.AwsSessionToken,
	}, nil
}

func (a *AwsRoleProviderMock) ExpiresAt() time.Time {
	return time.Time{}
}

func (a *AwsRoleProviderMock) RetrieveWithContext(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     a.AwsKey,
		SecretAccessKey: a.AwsSecret,
		SessionToken:    a.AwsSessionToken,
	}, nil
}

func (a *AwsRoleProviderMock) IsExpired() bool { return false }

// TODO: uncomment this test after figuring out how to create a mock compatible with WebIdentityRoleProvider
//func TestPopulationForAwsRoleToAssumeSetsValueOfKeys(t *testing.T) {
//	stateEnvVars := make(map[string]string)
//	commandEnvVars := make(map[string]string)
//
//	x := AwsRoleProviderMock{
//		AwsKey:          "statekey",
//		AwsSecret:       "stateSecret",
//		AwsSessionToken: "stateSessionToken",
//	}.(stscreds.WebIdentityRoleProvider)
//	job := Job{
//		StateEnvProvider: &x,
//		CommandEnvProvider: AwsRoleProviderMock{
//			AwsKey:          "commandkey",
//			AwsSecret:       "commandSecret",
//			AwsSessionToken: "commandSessionToken",
//		},
//
//		StateEnvVars:   stateEnvVars,
//		CommandEnvVars: commandEnvVars,
//	}
//
//	job.PopulateAwsCredentialsEnvVarsForJob()
//	assert.Equal(t, job.CommandEnvVars["AWS_ACCESS_KEY_ID"], "KEY")
//	assert.Equal(t, job.CommandEnvVars["AWS_SECRET_ACCESS_KEY"], "SECRET")
//	assert.Equal(t, job.CommandEnvVars["AWS_SESSION_TOKEN"], "TOKEN")
//	assert.Equal(t, job.StateEnvVars["AWS_ACCESS_KEY_ID"], "KEY")
//	assert.Equal(t, job.StateEnvVars["AWS_SECRET_ACCESS_KEY"], "SECRET")
//	assert.Equal(t, job.StateEnvVars["AWS_SESSION_TOKEN"], "TOKEN")
//}

func TestPopulationForNoAwsRoleToAssumeDoesNotSetValueOfKeys(t *testing.T) {
	stateEnvVars := make(map[string]string)
	commandEnvVars := make(map[string]string)

	job := Job{
		StateEnvVars:   stateEnvVars,
		CommandEnvVars: commandEnvVars,
	}

	job.PopulateAwsCredentialsEnvVarsForJob()
	assert.NotContains(t, job.CommandEnvVars, "AWS_ACCESS_KEY_ID")
	assert.NotContains(t, job.CommandEnvVars, "AWS_SECRET_ACCESS_KEY")
	assert.NotContains(t, job.CommandEnvVars, "AWS_SESSION_TOKEN")
	assert.NotContains(t, job.StateEnvVars, "AWS_ACCESS_KEY_ID")
	assert.NotContains(t, job.StateEnvVars, "AWS_SECRET_ACCESS_KEY")
	assert.NotContains(t, job.StateEnvVars, "AWS_SESSION_TOKEN")
}

func TestGetCognitoTokenGetsCreds(t *testing.T) {

	setupCognitoMocks()	
	mockProject := digger_config.Project{
		AwsCognitoOidcConfig: &digger_config.AwsCognitoOidcConfig{
			CognitoPoolId: "mock-identity-pool-id",
			AwsAccountId: "mock-aws-account-id",
			AwsRegion: "mock-aws-region",
		},
	}

	credentials, err := GetCognitoToken(*mockProject.AwsCognitoOidcConfig, "token.actions.githubusercontent.com")
	assert.Nil(t, err)
	assert.Equal(t, credentials, map[string]string{
		"AWS_ACCESS_KEY_ID":  		"mock-access-key-id",
		"AWS_SECRET_ACCESS_KEY": 	"mock-secret-access-key",
		"AWS_SESSION_TOKEN":   		"mock-session-token",
	});

	t.Cleanup(func() {
		defaultCognitoClient = nil
		defaultTokenFetcher = nil
	})	
}

func TestGetCognitoTokenReturnsErrorForMissingPoolId(t *testing.T) {
	setupCognitoMocks()
	mockProject := digger_config.Project{
		AwsCognitoOidcConfig: &digger_config.AwsCognitoOidcConfig{			
			AwsAccountId: "mock-aws-account-id",
			AwsRegion: "mock-aws-region",
		},
	}

	credentials, err := GetCognitoToken(*mockProject.AwsCognitoOidcConfig, "token.actions.githubusercontent.com")
	assert.NotNil(t, err)
	assert.Nil(t, credentials)
}

func TestGetCognitoTokenReturnsErrorForMissingAccountId(t *testing.T) {
	setupCognitoMocks()
	mockProject := digger_config.Project{
		AwsCognitoOidcConfig: &digger_config.AwsCognitoOidcConfig{
			CognitoPoolId: "mock-identity-pool-id",
			AwsRegion: "mock-aws-region",
		},
	}

	credentials, err := GetCognitoToken(*mockProject.AwsCognitoOidcConfig, "token.actions.githubusercontent.com")
	assert.NotNil(t, err)
	assert.Nil(t, credentials)
}

func TestGetCognitoTokenReturnsErrorForMissingRegion(t *testing.T) {
	setupCognitoMocks()
	mockProject := digger_config.Project{
		AwsCognitoOidcConfig: &digger_config.AwsCognitoOidcConfig{
			CognitoPoolId: "mock-identity-pool-id",
			AwsAccountId: "mock-aws-account-id",
		},
	}

	credentials, err := GetCognitoToken(*mockProject.AwsCognitoOidcConfig, "token.actions.githubusercontent.com")
	assert.NotNil(t, err)
	assert.Nil(t, credentials)
}

func TestParseRegionFromPoolId(t *testing.T) {
	region := parseRegionFromPoolId("us-east-1:00000000-0000-0000-0000-000000000000") // this is a fake guid generated locally. 
	assert.Equal(t, region, "us-east-1")
}

func TestGetAuthStrategyDefault(t *testing.T) {

	job := Job{
		ProjectName: "test-project",
		ProjectDir: "/tmp/test-project",		
	}
		
	strategy := job.GetAuthStrategy()
	assert.Equal(t, strategy, BackendConfig)
}

func TestGetAuthStrategyCognito(t *testing.T) {
	
	job := Job{
		ProjectName: "test-project",
		ProjectDir: "/tmp/test-project",
		CognitoOidcConfig: &digger_config.AwsCognitoOidcConfig{
			CognitoPoolId: "mock-identity-pool-id",
			AwsAccountId: "mock-aws-account-id",
			AwsRegion: "mock-aws-region",
		},
	}
		
	strategy := job.GetAuthStrategy()
	assert.Equal(t, strategy, Cognito)
}

func TestGetAuthStrategyTerragrunt(t *testing.T) {

	job := Job{
		ProjectName: "test-project",
		ProjectDir: "/tmp/test-project",
		Terragrunt: true,
	}
		
	strategy := job.GetAuthStrategy()
	assert.Equal(t, strategy, Terragrunt)
}

