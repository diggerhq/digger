package configuration

import (
	"digger/pkg/core/models"
	"github.com/dominikbraun/graph"
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
	dg, _, err := LoadDiggerConfig("", &FileSystemDirWalker{})
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

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
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

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "path/to/module/test", dg.GetDirectory("prod"))
}

func TestDefaultDiggerConfig(t *testing.T) {
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

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})

	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, 1, len(dg.Projects))
	assert.Equal(t, false, dg.AutoMerge)
	assert.Equal(t, true, dg.CollectUsageData)
	assert.Equal(t, 1, len(dg.Workflows))

	workflow := dg.Workflows["default"]
	assert.NotNil(t, workflow, "expected workflow to be not nil")
	assert.NotNil(t, workflow.Plan)
	assert.NotNil(t, workflow.Plan.Steps)

	assert.NotNil(t, workflow.Apply)
	assert.NotNil(t, workflow.Apply.Steps)
	assert.NotNil(t, workflow.EnvVars)
	assert.NotNil(t, workflow.Configuration)

	assert.Equal(t, "path/to/module/test", dg.GetDirectory("prod"))
}

func TestDiggerConfigDefaultWorkflow(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: prod
  branch: /main/
  dir: path/to/module/test
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "default", dg.Projects[0].Workflow)
	_, ok := dg.Workflows["default"]
	assert.True(t, ok)
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

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
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

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, models.Step{Action: "run", Value: "echo \"hello\"", Shell: ""}, dg.Workflows["myworkflow"].Plan.Steps[0], "parsed struct does not match expected struct")
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

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, []EnvVar{
		{Name: "TF_VAR_state", Value: "s3://mybucket/terraform.tfstate"},
	}, dg.Workflows["myworkflow"].EnvVars.State, "parsed struct does not match expected struct")
	assert.Equal(t, []EnvVar{
		{Name: "TF_VAR_command", Value: "plan"},
	}, dg.Workflows["myworkflow"].EnvVars.Commands, "parsed struct does not match expected struct")
}

func TestDefaultValuesForWorkflowConfiguration(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
  dir: .
  workflow: dev

workflows:
  dev:
    plan:
      steps:
        - run: rm -rf .terraform
        - init
        - plan:
          extra_args: ["-var-file=vars/dev.tfvars"]  
  default:
    plan:
      steps:
        - run: rm -rf .terraform
        - init
        - plan:
            extra_args: ["-var-file=vars/dev.tfvars"]

`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, models.Step{Action: "run", Value: "rm -rf .terraform", Shell: ""}, dg.Workflows["dev"].Plan.Steps[0], "parsed struct does not match expected struct")
	assert.Equal(t, models.Step{Action: "init", ExtraArgs: nil, Shell: ""}, dg.Workflows["dev"].Plan.Steps[1], "parsed struct does not match expected struct")
	assert.Equal(t, models.Step{Action: "plan", ExtraArgs: []string{"-var-file=vars/dev.tfvars"}, Shell: ""}, dg.Workflows["dev"].Plan.Steps[2], "parsed struct does not match expected struct")

	assert.Equal(t, models.Step{Action: "run", Value: "rm -rf .terraform", Shell: ""}, dg.Workflows["default"].Plan.Steps[0], "parsed struct does not match expected struct")
	assert.Equal(t, models.Step{Action: "init", ExtraArgs: nil, Shell: ""}, dg.Workflows["default"].Plan.Steps[1], "parsed struct does not match expected struct")
	assert.Equal(t, models.Step{Action: "plan", ExtraArgs: []string{"-var-file=vars/dev.tfvars"}, Shell: ""}, dg.Workflows["default"].Plan.Steps[2], "parsed struct does not match expected struct")
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

	dg, _, err := LoadDiggerConfig(tempDir, walker)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "test1", dg.Projects[0].Name)
	assert.Equal(t, "test2", dg.Projects[1].Name)
	assert.Equal(t, "dev/test1", dg.Projects[0].Dir)
	assert.Equal(t, "dev/test2", dg.Projects[1].Dir)
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

	dg, _, err := LoadDiggerConfig(tempDir, walker)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "test1", dg.Projects[0].Name)
	assert.Equal(t, "test2", dg.Projects[1].Name)
	assert.Equal(t, "dev/test1", dg.Projects[0].Dir)
	assert.Equal(t, "dev/test2", dg.Projects[1].Dir)
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

	dg, _, err := LoadDiggerConfig(tempDir, walker)
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

	_, _, err := LoadDiggerConfig(tempDir, walker)
	assert.ErrorContains(t, err, "no projects configuration found")
}

func TestDiggerConfigCustomWorkflow(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: my-first-app
  dir: app-one
  workflow: my_custom_workflow
workflows:
  my_custom_workflow:
    steps:
      - run: echo "run"
      - init: terraform init
      - plan: terraform plan
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger config to be not nil")
	assert.Equal(t, "my_custom_workflow", dg.Projects[0].Workflow)
	_, ok := dg.Workflows["my_custom_workflow"]
	assert.True(t, ok)
}

func TestDiggerConfigCustomWorkflowMissingParams(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	// missing workflow config
	diggerCfg := `
projects:
- name: my-first-app
  dir: app-one
  workflow: my_custom_workflow
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	_, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.Error(t, err, "failed to find workflow config 'my_custom_workflow' for project 'my-first-app'")

	// steps block is missing for workflows
	diggerCfg = `
projects:
- name: my-first-app
  dir: app-one
  workflow: my_custom_workflow
workflows:
  my_custom_workflow:
`
	deleteFile = createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	diggerConfig, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.Equal(t, "my_custom_workflow", diggerConfig.Projects[0].Workflow)
	workflow, ok := diggerConfig.Workflows["my_custom_workflow"]
	assert.True(t, ok)
	assert.NotNil(t, workflow)
	assert.NotNil(t, workflow.Plan)
	assert.NotNil(t, workflow.Apply)

}

func TestDiggerConfigMissingProjectsWorkflow(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: my-first-app
  dir: app-one
  workflow: my_custom_workflow
workflows:
  my_custom_workflow_no_one_use:
    steps:
      - run: echo "run"
      - init: terraform init
      - plan: terraform plan
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	_, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.Equal(t, "failed to find workflow config 'my_custom_workflow' for project 'my-first-app'", err.Error())

}

func TestDiggerConfigWithEmptyInitBlock(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: my-first-app
  dir: app-one
  workflow: default
workflows:
  default:
    steps:
      - run: echo "run"
      - init:
      - plan: terraform plan
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	_, _, err := LoadDiggerConfig(tempDir, &FileSystemDirWalker{})
	assert.Equal(t, "failed to find workflow config 'my_custom_workflow' for project 'my-first-app'", err.Error())

}

func TestDiggerConfigDependencyGraph(t *testing.T) {
	p1 := Project{
		Name:               "A",
		DependencyProjects: []string{"B", "C"},
	}

	p2 := Project{
		Name:               "B",
		DependencyProjects: []string{"C"},
	}

	p3 := Project{
		Name: "C",
	}

	p4 := Project{
		Name: "D",
	}

	p5 := Project{
		Name:               "E",
		DependencyProjects: []string{"A"},
	}

	p6 := Project{
		Name:               "F",
		DependencyProjects: []string{"A", "B"},
	}

	projects := []Project{p1, p2, p3, p4, p5, p6}

	g, err := CreateProjectDependencyGraph(projects)

	assert.NoError(t, err, "expected error to be nil")

	orderedProjects, _ := graph.StableTopologicalSort(g, func(s string, s2 string) bool {
		return s < s2
	})

	assert.Equal(t, 6, len(orderedProjects))
	assert.Equal(t, []string{"C", "D", "B", "A", "E", "F"}, orderedProjects)
}

func TestDiggerConfigDependencyGraph2(t *testing.T) {
	p1 := Project{
		Name:               "A",
		DependencyProjects: []string{"B", "C", "D"},
	}

	p2 := Project{
		Name:               "B",
		DependencyProjects: []string{"E", "F"},
	}

	p3 := Project{
		Name: "C",
		DependencyProjects: []string{
			"G",
		},
	}

	p4 := Project{
		Name: "D",
		DependencyProjects: []string{
			"H", "I",
		},
	}

	projects := []Project{p1, p2, p3, p4}

	g, err := CreateProjectDependencyGraph(projects)

	assert.NoError(t, err, "expected error to be nil")

	orderedProjects, _ := graph.StableTopologicalSort(g, func(s string, s2 string) bool {
		return s > s2
	})

	assert.Equal(t, 9, len(orderedProjects))
	assert.Equal(t, []string{"I", "H", "G", "F", "E", "D", "C", "B", "A"}, orderedProjects)
}

func TestDiggerConfigDependencyGraphWithCyclesFails(t *testing.T) {
	p1 := Project{
		Name:               "A",
		DependencyProjects: []string{"B"},
	}

	p2 := Project{
		Name:               "B",
		DependencyProjects: []string{"C"},
	}

	p3 := Project{
		Name: "C",
		DependencyProjects: []string{
			"A",
		},
	}

	projects := []Project{p1, p2, p3}

	_, err := CreateProjectDependencyGraph(projects)

	assert.Error(t, err, "expected error on cycle")
	assert.Equal(t, "edge would create a cycle", err.Error())
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
