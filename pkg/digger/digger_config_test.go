package digger

import (
	"log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setUp() (string, func()) {
	tempDir := createTempDir()
	return tempDir, func() {
		deleteTempDir(tempDir)
	}
}

func TestDiggerConfigFileDoesNotExist(t *testing.T) {
	dg, err := NewDiggerConfig("", &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be not nil")
	assert.Equal(t, dg.Projects[0].Name, "default", "expected default project to have name 'default'")
	assert.Equal(t, dg.Projects[0].Dir, ".", "expected default project dir to be '.'")
}

func TestDiggerConfigWhenMultipleConfigExist(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	_, err := os.Create(path.Join(tempDir, "digger.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Create(path.Join(tempDir, "digger.yml"))
	if err != nil {
		t.Fatal(err)
	}

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.Error(t, err, "expected error to be returned")
	assert.ErrorContains(t, err, ErrDiggerConfigConflict.Error(), "expected error to match target error")
	assert.Nil(t, dg, "expected diggerConfig to be nil")
}

func TestDiggerConfigWhenOnlyYamlExists(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: prod
  branch: /main/
  dir: path/to/module/test
  workspace: default
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "path/to/module/test", dg.GetDirectory("prod"))
}

func TestDiggerConfigWhenOnlyYmlExists(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
  branch: /main/
  dir: path/to/module
  workspace: default
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "path/to/module", dg.GetDirectory("dev"))
}

func TestDefaultValuesForWorkflowConfiguration(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
workflows:
  default:
    plan:
      steps:
      - init
      - plan:
          extra_args: ["-var-file=terraform.tfvars"]
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, Step{"init", nil}, dg.Workflows["default"].Plan.Steps[0], "expected step name to be 'init'")
	assert.Equal(t, Step{"plan", []string{"-var-file=terraform.tfvars"}}, dg.Workflows["default"].Plan.Steps[1], "expected step name to be 'init'")
}

func TestDiggerGenerateProjects(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  include: dev/*
  exclude: dev/project
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()

	walker := &MockDirWalker{}
	walker.Files = append(walker.Files, "dev/test1")
	walker.Files = append(walker.Files, "dev/test2")
	walker.Files = append(walker.Files, "dev/project")
	walker.Files = append(walker.Files, "testtt")

	dg, err := NewDiggerConfig(tempDir, walker)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "dev/test1", dg.Projects[0].Name)
	assert.Equal(t, "dev/test2", dg.Projects[1].Name)
	assert.Equal(t, 2, len(dg.Projects))
	//assert.Equal(t, "path/to/module", dg.GetDirectory("dev"))
}

func createTempDir() string {
	dir, err := os.MkdirTemp("", "tmp")
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func deleteTempDir(name string) {
	err := os.RemoveAll(name)
	if err != nil {
		log.Fatal(err)
	}
}

func createFile(filepath string, content string) func() {
	f, err := os.Create(filepath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.WriteString(content)
	if err != nil {
		log.Fatal(err)
	}

	return func() {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}
