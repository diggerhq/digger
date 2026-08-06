package execution

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteTerraformPlan(t *testing.T) {
	dir := CreateTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			panic(err)
		}
	}(dir)

	CreateValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir, Workspace: "dev"}
	tf.Init([]string{}, map[string]string{})
	_, _, _, err := tf.Plan([]string{}, map[string]string{}, "", nil)
	assert.NoError(t, err)
}

func TestExecuteTerraformApply(t *testing.T) {
	dir := CreateTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			panic(err)
		}
	}(dir)

	CreateValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir, Workspace: "dev"}
	tf.Init([]string{}, map[string]string{})
	_, _, _, err := tf.Plan([]string{}, map[string]string{}, "", nil)
	assert.NoError(t, err)
}

func TestExecuteTerraformApplyDefaultWorkspace(t *testing.T) {
	dir := CreateTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			panic(err)
		}
	}(dir)

	CreateValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir, Workspace: "default"}
	tf.Init([]string{}, map[string]string{})
	var planArgs []string
	planArgs = append(planArgs, "-out", "plan.tfplan")
	tf.Plan(planArgs, map[string]string{}, "", nil)
	plan := "plan.tfplan"
	_, _, err := tf.Apply([]string{}, &plan, map[string]string{})
	assert.NoError(t, err)
}

func TestRedactSecrets(t *testing.T) {
	secrets := []string{
		"-backend-config=access_key=xxx",
		"-backend-config=secret_key=yyy",
		"-backend-config=token=zzz",
	}
	redactedSecrets := RedactSecrets(secrets)
	assert.Equal(t, redactedSecrets[0], "-backend-config=access_key=<REDACTED>")
	assert.Equal(t, redactedSecrets[1], "-backend-config=secret_key=<REDACTED>")
	assert.Equal(t, redactedSecrets[2], "-backend-config=token=<REDACTED>")
}

func TestRedactPlanSecrets(t *testing.T) {
	rawPlan := `- env {
    - name  = "SOME_API_KEY" -> null
    - value = "<SYNTHETIC-SECRET-VALUE>" -> null
  }`
	redacted := RedactPlanSecrets(rawPlan)
	assert.Contains(t, redacted, `name  = "SOME_API_KEY"`)
	assert.Contains(t, redacted, `value = "<REDACTED>" -> "<REDACTED>"`)
	assert.NotContains(t, redacted, "<SYNTHETIC-SECRET-VALUE>")

	directAttr := `api_key = "super-secret-key-value"`
	redactedAttr := RedactPlanSecrets(directAttr)
	assert.Equal(t, `api_key = "<REDACTED>"`, redactedAttr)
}
