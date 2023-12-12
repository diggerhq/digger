package digger

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/stretchr/testify/assert"
	"testing"
)

type AwsRoleProviderMock struct{}

func (a AwsRoleProviderMock) GetKeysFromRole(role string) (*credentials.Value, error) {
	return &credentials.Value{
		AccessKeyID:     "KEY",
		SecretAccessKey: "SECRET",
		SessionToken:    "TOKEN",
		ProviderName:    "PROVIDER",
	}, nil
}

func TestPopulationForAwsRoleToAssumeSetsValueOfKeys(t *testing.T) {
	stateEnvVars := make(map[string]string)
	commandEnvVars := make(map[string]string)

	job := orchestrator.Job{
		AwsRoleToAssume: "arn:blablabla",
		StateEnvVars:    stateEnvVars,
		CommandEnvVars:  commandEnvVars,
	}

	provider := AwsRoleProviderMock{}
	PopulateAwsCredentialsEnvVarsForJob(&job, provider)
	assert.Equal(t, job.CommandEnvVars["AWS_ACCESS_KEY_ID"], "KEY")
	assert.Equal(t, job.CommandEnvVars["AWS_SECRET_ACCESS_KEY"], "SECRET")
	assert.Equal(t, job.CommandEnvVars["AWS_SESSION_TOKEN"], "TOKEN")
	assert.Equal(t, job.StateEnvVars["AWS_ACCESS_KEY_ID"], "KEY")
	assert.Equal(t, job.StateEnvVars["AWS_SECRET_ACCESS_KEY"], "SECRET")
	assert.Equal(t, job.StateEnvVars["AWS_SESSION_TOKEN"], "TOKEN")
}

func TestPopulationForNoAwsRoleToAssumeDoesNotSetValueOfKeys(t *testing.T) {
	stateEnvVars := make(map[string]string)
	commandEnvVars := make(map[string]string)

	job := orchestrator.Job{
		AwsRoleToAssume: "",
		StateEnvVars:    stateEnvVars,
		CommandEnvVars:  commandEnvVars,
	}

	provider := AwsRoleProviderMock{}
	PopulateAwsCredentialsEnvVarsForJob(&job, provider)
	assert.NotContains(t, job.CommandEnvVars, "AWS_ACCESS_KEY_ID")
	assert.NotContains(t, job.CommandEnvVars, "AWS_SECRET_ACCESS_KEY")
	assert.NotContains(t, job.CommandEnvVars, "AWS_SESSION_TOKEN")
	assert.NotContains(t, job.StateEnvVars, "AWS_ACCESS_KEY_ID")
	assert.NotContains(t, job.StateEnvVars, "AWS_SECRET_ACCESS_KEY")
	assert.NotContains(t, job.StateEnvVars, "AWS_SESSION_TOKEN")
}
