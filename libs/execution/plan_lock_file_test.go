package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectPathProviderPlanLockFilePaths(t *testing.T) {
	prNumber := 123
	provider := ProjectPathProvider{
		PRNumber:         &prNumber,
		ProjectPath:      "projects/app",
		ProjectNamespace: "prod/aws",
		ProjectName:      "network",
	}

	assert.Equal(t, "projects/app/.terraform.lock.hcl", provider.LocalPlanLockFilePath(".terraform.lock.hcl"))
	assert.Equal(t, "prod-aws-123-network.terraform.lock.hcl", provider.StoredPlanLockFilePath(".terraform.lock.hcl"))
	assert.Equal(t, "prod-aws-123-network.tofu.lock.hcl", provider.StoredPlanLockFilePath("locks/tofu.lock.hcl"))
	assert.Equal(t, "projects/app/prod-aws-123-network.terraform.lock.hcl", localStoredPlanLockFilePath(provider, ".terraform.lock.hcl"))
}

func TestValidatePlanLockFilePath(t *testing.T) {
	assert.NoError(t, ValidatePlanLockFilePath(".terraform.lock.hcl"))
	assert.NoError(t, ValidatePlanLockFilePath("locks/tofu.lock.hcl"))
	assert.Error(t, ValidatePlanLockFilePath("/tmp/.terraform.lock.hcl"))
	assert.Error(t, ValidatePlanLockFilePath("../.terraform.lock.hcl"))
	assert.Error(t, ValidatePlanLockFilePath("locks/../../.terraform.lock.hcl"))
	assert.Error(t, ValidatePlanLockFilePath(`locks\..\..\.terraform.lock.hcl`))
	assert.Error(t, ValidatePlanLockFilePath(".."))
}
