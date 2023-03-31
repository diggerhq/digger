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

	tf := Terraform{WorkingDir: dir, Workspace: "default"}
	_, _, _, err := tf.Plan([]string{}, []string{})
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

	tf := Terraform{WorkingDir: dir, Workspace: "default"}
	_, _, err := tf.Apply([]string{}, []string{})
	assert.NoError(t, err)
}
