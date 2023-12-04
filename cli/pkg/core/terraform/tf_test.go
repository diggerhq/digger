package terraform

import (
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func TestExecuteTerraformPlan(t *testing.T) {
	dir := CreateTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	CreateValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir, Workspace: "dev"}
	tf.Init([]string{}, map[string]string{})
	_, _, _, err := tf.Plan([]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestExecuteTerraformApply(t *testing.T) {
	dir := CreateTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	CreateValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir, Workspace: "dev"}
	tf.Init([]string{}, map[string]string{})
	_, _, _, err := tf.Plan([]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestExecuteTerraformApplyDefaultWorkspace(t *testing.T) {
	dir := CreateTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	CreateValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir, Workspace: "default"}
	tf.Init([]string{}, map[string]string{})
	var planArgs []string
	planArgs = append(planArgs, "-out", "plan.tfplan")
	tf.Plan(planArgs, map[string]string{})
	plan := "plan.tfplan"
	_, _, err := tf.Apply([]string{}, &plan, map[string]string{})
	assert.NoError(t, err)
}
