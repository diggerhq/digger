package digger

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/diggerhq/digger/cli/pkg/aws/envprovider"
	"github.com/diggerhq/digger/libs/orchestrator"
	"log"
)

func PopulateAwsCredentialsEnvVarsForJob(job *orchestrator.Job) (orchestrator.Job, error) {
	awsRoleToAssume := job.AwsRoleToAssume
	if awsRoleToAssume == "" {
		return *job, nil
	}

	log.Println(awsRoleToAssume)
	creds, err := envprovider.GetKeysFromRole(awsRoleToAssume)
	if err != nil {
		log.Printf("Failed to get keys from role: %v", err)
		return *job, fmt.Errorf("Failed to get keys from role: %v", err)
	}
	populateKeys := func(envs map[string]string, creds *credentials.Value) map[string]string {
		envs["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
		envs["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
		envs["AWS_SESSION_TOKEN"] = creds.SessionToken
		return envs
	}
	job.CommandEnvVars = populateKeys(job.CommandEnvVars, creds)
	job.StateEnvVars = populateKeys(job.StateEnvVars, creds)

	return *job, nil
}
