package digger

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiggerConfigFileDoesNotExist(t *testing.T) {
	dg, err := NewDiggerConfig("")
	assert.NoError(t, err, "expected error to be not nil")
	assert.Equal(t, dg.Projects[0].Name, "default", "expected default project to have name 'default'")
	assert.Equal(t, dg.Projects[0].Dir, ".", "expected default project dir to be '.'")
}

func TestMultipleDiggerConfigFileExist(t *testing.T) {
	tempDir := CreateTempDir()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(tempDir)
	os.Create(tempDir + "digger.yaml")
	os.Create(tempDir + "digger.yml")

	dg, err := NewDiggerConfig(tempDir)
	assert.Error(t, err, "expected error to be returned")
	assert.Nil(t, dg, "expected diggerConfig to be nil")
}

func TestDefaultValuesForWorkflowConfiguration(t *testing.T) {
	tempDir := CreateTempDir()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(tempDir)

	f, err := os.Create(tempDir + "/digger.yml")
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	digger_yml := `
projects:
- name: dev
  branch: /main/
  dir: .
  workspace: default
`
	_, err2 := f.WriteString(digger_yml)
	if err2 != nil {
		log.Fatal(err2)
	}

	dg, err := NewDiggerConfig(tempDir)
	assert.NoError(t, err, "expected error to be not nil")
	assert.Equal(t, dg.Projects[0].WorkflowConfiguration.OnPullRequestPushed[0], "digger plan")
	assert.Equal(t, dg.Projects[0].WorkflowConfiguration.OnPullRequestClosed[0], "digger unlock")
	assert.Equal(t, dg.Projects[0].WorkflowConfiguration.OnCommitToDefault[0], "digger apply")
}

func CreateTempDir() string {
	dir, err := os.MkdirTemp("", "tmp")
	if err != nil {
		log.Fatal(err)
	}
	return dir
}
