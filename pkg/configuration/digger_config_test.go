package configuration

import (
	"log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockDirWalker struct {
	Files []string
}

func (walker *MockDirWalker) GetDirs(workingDir string) ([]string, error) {

	return walker.Files, nil
}

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

func TestCustomCommandsConfiguration(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
  dir: infra/dev
  workflow: myworkflow

workflows:
  myworkflow:
    plan:
      steps:
      - run: echo "hello"
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, Step{Action: "run", Value: "echo \"hello\"", Shell: ""}, dg.Workflows["myworkflow"].Plan.Steps[0], "parsed struct does not match expected struct")
}

func TestEnvVarsConfiguration(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
  branch: /main/
  dir: .
  workspace: default
  terragrunt: false
  workflow: myworkflow
workflows:
  myworkflow:
    plan:
      steps:
      - init:
          extra_args: ["-lock=false"]
      - plan:
          extra_args: ["-lock=false"]
      - run: echo "hello"
    apply:
      steps:
      - apply:
          extra_args: ["-lock=false"]
    workflow_configuration:
      on_pull_request_pushed: [digger plan]
      on_pull_request_closed: [digger unlock]
      on_commit_to_default: [digger apply]
    env_vars:
      state:
      - name: TF_VAR_state
        value: s3://mybucket/terraform.tfstate
      commands:
      - name: TF_VAR_command
        value: plan
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, []EnvVarConfig{
		{Name: "TF_VAR_state", Value: "s3://mybucket/terraform.tfstate"},
	}, dg.Workflows["myworkflow"].EnvVars.State, "parsed struct does not match expected struct")
	assert.Equal(t, []EnvVarConfig{
		{Name: "TF_VAR_command", Value: "plan"},
	}, dg.Workflows["myworkflow"].EnvVars.Commands, "parsed struct does not match expected struct")
}

func TestDefaultValuesForWorkflowConfiguration(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
workflows:
  default:
    plan:
      steps:
      - init
      - plan:
        extra_args: ["-var-file=terraform.tfvars"]
      - run: echo "hello"
        shell: zsh
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, err := NewDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, Step{Action: "init", ExtraArgs: nil, Shell: ""}, dg.Workflows["default"].Plan.Steps[0], "parsed struct does not match expected struct")
	assert.Equal(t, Step{Action: "plan", ExtraArgs: []string{"-var-file=terraform.tfvars"}, Shell: ""}, dg.Workflows["default"].Plan.Steps[1], "parsed struct does not match expected struct")
	assert.Equal(t, Step{Action: "run", Value: "echo \"hello\"", Shell: "zsh"}, dg.Workflows["default"].Plan.Steps[2], "parsed struct does not match expected struct")
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
	walker.Files = append(walker.Files, "dev")
	walker.Files = append(walker.Files, "dev/test1")
	walker.Files = append(walker.Files, "dev/test2")
	walker.Files = append(walker.Files, "dev/project")
	walker.Files = append(walker.Files, "testtt")

	dg, err := NewDiggerConfig(tempDir, walker)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "test1", dg.Projects[0].Name)
	assert.Equal(t, "test2", dg.Projects[1].Name)
	assert.Equal(t, tempDir+"/dev/test1", dg.Projects[0].Dir)
	assert.Equal(t, tempDir+"/dev/test2", dg.Projects[1].Dir)
	assert.Equal(t, 2, len(dg.Projects))
}

func TestDiggerGenerateProjectsWithSubDirs(t *testing.T) {
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
	walker.Files = append(walker.Files, "dev")
	walker.Files = append(walker.Files, "dev/test1")
	walker.Files = append(walker.Files, "dev/test1/utils")
	walker.Files = append(walker.Files, "dev/test2")
	walker.Files = append(walker.Files, "dev/project")
	walker.Files = append(walker.Files, "testtt")

	dg, err := NewDiggerConfig(tempDir, walker)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "test1", dg.Projects[0].Name)
	assert.Equal(t, "test2", dg.Projects[1].Name)
	assert.Equal(t, tempDir+"/dev/test1", dg.Projects[0].Dir)
	assert.Equal(t, tempDir+"/dev/test2", dg.Projects[1].Dir)
	assert.Equal(t, 2, len(dg.Projects))
}

func TestDiggerGenerateProjectsIgnoreSubdirs(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  include: dev
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()

	walker := &MockDirWalker{}
	walker.Files = append(walker.Files, "dev")
	walker.Files = append(walker.Files, "dev/test1")
	walker.Files = append(walker.Files, "dev/test1/utils")
	walker.Files = append(walker.Files, "dev/test2")
	walker.Files = append(walker.Files, "dev/project")
	walker.Files = append(walker.Files, "testtt")

	dg, err := NewDiggerConfig(tempDir, walker)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "dev", dg.Projects[0].Name)
	assert.Equal(t, 1, len(dg.Projects))
}

func TestMissingProjectsReturnsError(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()
	walker := &MockDirWalker{}

	_, err := NewDiggerConfig(tempDir, walker)
	assert.ErrorContains(t, err, "no projects configuration found")
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
