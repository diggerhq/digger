package digger_config

import (
	"fmt"
	"log"
	"os"
	"path"
	"testing"

	"github.com/dominikbraun/graph"
	"github.com/go-git/go-git/v5"

	"github.com/stretchr/testify/assert"
)

func setUp() (string, func()) {
	tempDir := createTempDir()
	return tempDir, func() {
		deleteTempDir(tempDir)
	}
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.Error(t, err, "expected error to be returned")
	assert.ErrorContains(t, err, ErrDiggerConfigConflict.Error(), "expected error to match target error")
	assert.Nil(t, dg, "expected diggerConfig to be nil")
}

func TestDiggerConfigWhenCustomFileName(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	os.Setenv("DIGGER_FILENAME", "digger-custom.yml")

	_, err := os.Create(path.Join(tempDir, "digger-custom.yml"))
	if err != nil {
		t.Fatal(err)
	}

	configPath, err := retrieveConfigFile(tempDir)
	fmt.Println(configPath)

	assert.Nil(t, err)
	assert.Equal(t, configPath, path.Join(tempDir, "digger-custom.yml"))

	os.Unsetenv("DIGGER_FILENAME")

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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "path/to/module/test", dg.GetDirectory("prod"))
}

func TestNoDiggerYaml(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	terraformFile := ""
	deleteFile := createFile(path.Join(tempDir, "main.tf"), terraformFile)
	defer deleteFile()

	os.Chdir(tempDir)
	_, _, _, err := LoadDiggerConfig("./", true, nil)
	assert.Error(t, err, "expected error since digger.yml and digger.yaml is missing")
}

func TestDefaultDiggerConfig(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: prod
  branch: /main/
  dir: path/to/module/test
  aws_role_to_assume:
    state: "arn://abc:xyz:state"
    command: "arn://abc:xyz:cmd"
  workspace: default
  workflow_file: "test.yml"
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	fmt.Printf("%v", err)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, 1, len(dg.Projects))
	assert.Equal(t, false, dg.AutoMerge)
	assert.Equal(t, true, dg.Telemetry)
	assert.Equal(t, false, dg.TraverseToNestedProjects)
	assert.Equal(t, 1, len(dg.Workflows))

	assert.Equal(t, "prod", dg.Projects[0].Name)
	assert.Equal(t, "test.yml", dg.Projects[0].WorkflowFile)
	assert.Equal(t, "path/to/module/test", dg.Projects[0].Dir)
	assert.Equal(t, "arn://abc:xyz:cmd", dg.Projects[0].AwsRoleToAssume.Command)
	assert.Equal(t, "arn://abc:xyz:state", dg.Projects[0].AwsRoleToAssume.State)

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

func TestDiggerConfigOneRole(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: prod
  branch: /main/
  aws_role_to_assume:
    command: "arn://abc:xyz:cmd"
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	fmt.Printf("%v", err)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "arn://abc:xyz:cmd", dg.Projects[0].AwsRoleToAssume.Command)
	assert.Equal(t, "arn://abc:xyz:cmd", dg.Projects[0].AwsRoleToAssume.State)
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.Equal(t, Step{Action: "run", Value: "rm -rf .terraform", Shell: ""}, dg.Workflows["dev"].Plan.Steps[0], "parsed struct does not match expected struct")
	assert.Equal(t, Step{Action: "init", ExtraArgs: nil, Shell: ""}, dg.Workflows["dev"].Plan.Steps[1], "parsed struct does not match expected struct")
	assert.Equal(t, Step{Action: "plan", ExtraArgs: []string{"-var-file=vars/dev.tfvars"}, Shell: ""}, dg.Workflows["dev"].Plan.Steps[2], "parsed struct does not match expected struct")

	assert.Equal(t, Step{Action: "run", Value: "rm -rf .terraform", Shell: ""}, dg.Workflows["default"].Plan.Steps[0], "parsed struct does not match expected struct")
	assert.Equal(t, Step{Action: "init", ExtraArgs: nil, Shell: ""}, dg.Workflows["default"].Plan.Steps[1], "parsed struct does not match expected struct")
	assert.Equal(t, Step{Action: "plan", ExtraArgs: []string{"-var-file=vars/dev.tfvars"}, Shell: ""}, dg.Workflows["default"].Plan.Steps[2], "parsed struct does not match expected struct")
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
	dirsToCreate := []string{"dev/test1", "dev/test2", "dev/project", "testtt"}

	for _, dir := range dirsToCreate {
		err := os.MkdirAll(path.Join(tempDir, dir), os.ModePerm)
		defer createFile(path.Join(tempDir, dir, "main.tf"), "")()
		assert.NoError(t, err, "expected error to be nil")
	}

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "dev_test1", dg.Projects[0].Name)
	assert.Equal(t, "dev_test2", dg.Projects[1].Name)
	assert.Equal(t, "dev/test1", dg.Projects[0].Dir)
	assert.Equal(t, "dev/test2", dg.Projects[1].Dir)
	assert.Equal(t, 2, len(dg.Projects))
}

func TestDiggerGenerateProjectsWithSubDirs(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  include: dev/**
  exclude: dev/project
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()
	dirsToCreate := []string{
		"dev/test1/utils",
		"dev/test2",
		"dev/project",
		"testtt",
	}
	for _, dir := range dirsToCreate {
		err := os.MkdirAll(path.Join(tempDir, dir), os.ModePerm)
		defer createFile(path.Join(tempDir, dir, "main.tf"), "")()
		assert.NoError(t, err, "expected error to be nil")
	}

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "dev_test1_utils", dg.Projects[0].Name)
	assert.Equal(t, "dev_test2", dg.Projects[1].Name)
	assert.Equal(t, "dev/test1/utils", dg.Projects[0].Dir)
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
	dirsToCreate := []string{
		"dev",
		"dev/test1",
		"dev/test1/utils",
		"dev/test2",
		"dev/project",
		"testtt",
	}
	for _, dir := range dirsToCreate {
		err := os.MkdirAll(path.Join(tempDir, dir), os.ModePerm)
		defer createFile(path.Join(tempDir, dir, "main.tf"), "")()
		assert.NoError(t, err, "expected error to be nil")
	}
	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
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
	config, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.Nil(t, err)
	assert.Equal(t, len(config.Projects), 0)
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

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "my_custom_workflow", dg.Projects[0].Workflow)
	_, ok := dg.Workflows["my_custom_workflow"]
	assert.True(t, ok)
}

func TestDiggerConfigCustomWorkflowMissingParams(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	// missing workflow digger_config
	diggerCfg := `
projects:
- name: my-first-app
  dir: app-one
  workflow: my_custom_workflow
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	_, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.Error(t, err, "failed to find workflow digger_config 'my_custom_workflow' for project 'my-first-app'")

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

	diggerConfig, _, _, err := LoadDiggerConfig(tempDir, true, nil)
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

	_, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.Equal(t, "failed to find workflow digger_config 'my_custom_workflow' for project 'my-first-app'", err.Error())

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
    plan:
      steps:
      - init:
      - plan:
        extra_args: ["-var-file=$ENV_NAME"]
`
	deleteFile := createFile(path.Join(tempDir, "digger.yaml"), diggerCfg)
	defer deleteFile()

	_, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.Nil(t, err)
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

func TestDiggerYamlDependencyGraph(t *testing.T) {
	diggerCfg := `
projects:
- name: my-first-app
  dir: app-one
  workflow: default
- name: my-second-app
  dir: app-two
  workflow: default
  depends_on: ["my-first-app"]
`
	dg, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "default", dg.Projects[0].Workflow)

	assert.Equal(t, "my-first-app", dg.Projects[0].Name)
	assert.Equal(t, "my-second-app", dg.Projects[1].Name)

	assert.Equal(t, "my-first-app", dg.Projects[1].DependencyProjects[0])
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

	p5 := Project{
		Name: "E",
	}

	p6 := Project{
		Name: "F",
	}

	p7 := Project{
		Name: "G",
	}
	p8 := Project{
		Name: "H",
	}

	p9 := Project{
		Name: "I",
	}

	projects := []Project{p1, p2, p3, p4, p5, p6, p7, p8, p9}

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

func TestLoadDiggerConfigYamlFromString(t *testing.T) {
	diggerCfg := `
projects:
- name: prod
  branch: /main/
  dir: path/to/module/test
`

	dg, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "default", dg.Projects[0].Workflow)
	_, ok := dg.Workflows["default"]
	assert.True(t, ok)
}

func TestDiggerConfigMissingProjectsWorkflowConfiguration(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()
	tests := []struct {
		name      string
		diggerCfg string
		wantErr   string
	}{
		{
			name: "on_pull_request_pushed empty",
			diggerCfg: `
projects:
- name: dev
  branch: /main/
  dir: .
  workspace: default
  terragrunt: false
  workflow: myworkflow
workflows:
  myworkflow:
    workflow_configuration:
      on_pull_request_pushed:
      on_pull_request_closed: [digger unlock]
      on_commit_to_default: [digger apply]
`,
			wantErr: "workflow_configuration.on_pull_request_pushed is required",
		},
		{
			name: "on_pull_request_closed empty",
			diggerCfg: `
projects:
- name: dev
  branch: /main/
  dir: .
  workspace: default
  terragrunt: false
  workflow: myworkflow
workflows:
  myworkflow:
    workflow_configuration:
      on_pull_request_pushed: [digger plan]
      on_pull_request_closed:
      on_commit_to_default: [digger apply]
`,
			wantErr: "workflow_configuration.on_pull_request_closed is required",
		},
		{
			name: "on_commit_to_default empty",
			diggerCfg: `
projects:
- name: dev
  branch: /main/
  dir: .
  workspace: default
  terragrunt: false
  workflow: myworkflow
workflows:
  myworkflow:
    workflow_configuration:
      on_pull_request_pushed: [digger plan]
      on_pull_request_closed: [digger unlock]
      on_commit_to_default:
`,
			wantErr: "workflow_configuration.on_commit_to_default is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteFile := createFile(path.Join(tempDir, "digger.yaml"), tt.diggerCfg)
			defer deleteFile()
			_, _, _, err := LoadDiggerConfig(tempDir, true, nil)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
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
		fmt.Printf("deleteTempDir error, %v", err.Error())
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

func createAndCloseFile(filepath string, content string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}

	_, err = f.WriteString(content)
	if err != nil {
		return err
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close file %v\n", f.Name())
		}
	}(f)
	return nil
}

func TestDiggerGenerateProjectsMultiplePatterns(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  blocks:
    - include: dev/*
      exclude: dev/project
      include_patterns: ["modules/**"]
      exclude_patterns: ["modules/dontincludeme/**"]
      workflow: dev_workflow
      workspace: devdev
    - include: prod/*
      exclude: prod/project
      workflow: prod_workflow
workflows:
  dev_workflow:
    steps:
      - run: echo "run"
      - init: terraform init
      - plan: terraform plan
  prod_workflow:
    steps:
      - run: echo "run"
      - init: terraform init
      - plan: terraform plan
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()
	dirsToCreate := []string{"dev/test1", "dev/test2", "dev/project", "testtt", "prod/one"}

	for _, dir := range dirsToCreate {
		err := os.MkdirAll(path.Join(tempDir, dir), os.ModePerm)
		defer createFile(path.Join(tempDir, dir, "main.tf"), "")()
		assert.NoError(t, err, "expected error to be nil")
	}

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "dev_test1", dg.Projects[0].Name)
	assert.Equal(t, "dev_test2", dg.Projects[1].Name)
	assert.Equal(t, "prod_one", dg.Projects[2].Name)
	assert.Equal(t, "dev_workflow", dg.Projects[0].Workflow)
	assert.Equal(t, "dev_workflow", dg.Projects[1].Workflow)
	assert.Equal(t, "prod_workflow", dg.Projects[2].Workflow)
	assert.Equal(t, "devdev", dg.Projects[0].Workspace)
	assert.Equal(t, "devdev", dg.Projects[1].Workspace)
	assert.Equal(t, "default", dg.Projects[2].Workspace)
	assert.Equal(t, "dev/test1", dg.Projects[0].Dir)
	assert.Equal(t, "dev/test2", dg.Projects[1].Dir)
	assert.Equal(t, "prod/one", dg.Projects[2].Dir)
	assert.Equal(t, []string{"modules/**"}, dg.Projects[0].IncludePatterns)
	assert.Equal(t, []string{"modules/**"}, dg.Projects[1].IncludePatterns)
	assert.Nil(t, dg.Projects[2].IncludePatterns)
	assert.Equal(t, []string{"modules/dontincludeme/**"}, dg.Projects[0].ExcludePatterns)
	assert.Equal(t, []string{"modules/dontincludeme/**"}, dg.Projects[1].ExcludePatterns)
	assert.Nil(t, dg.Projects[2].ExcludePatterns)

	assert.Equal(t, 3, len(dg.Projects))
}

func TestDiggerGenerateProjectsWithOpenTofu(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  blocks:
    - include: tofu/*
      opentofu: true
    - include: terraform/*
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()
	dirsToCreate := []string{"tofu/test1", "tofu/test2", "terraform/one"}

	for _, dir := range dirsToCreate {
		err := os.MkdirAll(path.Join(tempDir, dir), os.ModePerm)
		defer createFile(path.Join(tempDir, dir, "main.tf"), "")()
		assert.NoError(t, err, "expected error to be nil")
	}

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, "tofu_test1", dg.Projects[0].Name)
	assert.Equal(t, "tofu_test2", dg.Projects[1].Name)
	assert.Equal(t, "terraform_one", dg.Projects[2].Name)
	assert.Equal(t, true, dg.Projects[0].OpenTofu)
	assert.Equal(t, true, dg.Projects[1].OpenTofu)
	assert.Equal(t, false, dg.Projects[2].OpenTofu)
	assert.Equal(t, "tofu/test1", dg.Projects[0].Dir)
	assert.Equal(t, "tofu/test2", dg.Projects[1].Dir)
	assert.Equal(t, "terraform/one", dg.Projects[2].Dir)
	assert.Equal(t, 3, len(dg.Projects))
}

// TestDiggerGenerateProjectsEmptyParameters test if missing parameters for generate_projects are handled correctly
func TestDiggerGenerateProjectsEmptyParameters(t *testing.T) {
	_, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
`
	config, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(config.Projects))
}

// TestDiggerGenerateProjectsTooManyParameters include/exclude and blocks of include/exclude can't be used together
func TestDiggerGenerateProjectsTooManyParameters(t *testing.T) {
	_, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  include: dev/*
  exclude: dev/project
  blocks:
    - include: dev/*
      exclude: dev/project
      workflow: default
    - include: prod/*
      exclude: prod/project
      workflow: default
`
	_, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	assert.Error(t, err)
	assert.Equal(t, "if include/exclude patterns are used for project generation, blocks of include/exclude can't be used", err.Error())
}

func TestDiggerTerragruntProjects(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
  dir: .
  terragrunt: true
`
	defer createFile(path.Join(tempDir, "digger.yml"), diggerCfg)()
	defer createFile(path.Join(tempDir, "main.tf"), "resource \"null_resource\" \"test4\" {}")()
	defer createFile(path.Join(tempDir, "terragrunt.hcl"), "terraform {}")()

	_, config, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err)

	print(config)
}

func TestDiggerTerragruntProjectGenerationChainedDependencies(t *testing.T) {
	// based on https://github.com/transcend-io/terragrunt-atlantis-config/tree/master/test_examples/chained_dependencies
	// TODO: this test is a bit slow because we are cloning the whole repo, maybe we can copy it to a smaller repo
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  terragrunt: true
  terragrunt_parsing:
    parallel: true
    createProjectName: true
    defaultWorkflow: default
`

	repoUrl := "https://github.com/diggerhq/terragrunt-atlantis-config-examples.git"
	_, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})
	assert.NoError(t, err)

	// example dir: /test_examples/chained_dependencies
	projectDir := tempDir + "/chained_dependencies"

	err = createAndCloseFile(path.Join(projectDir, "digger.yml"), diggerCfg)
	assert.NoError(t, err)
	_, _, _, err = LoadDiggerConfig(projectDir, true, nil)
	assert.NoError(t, err)
}

func TestDiggerTerragruntProjectGenerationBasicModule(t *testing.T) {
	// based on https://github.com/transcend-io/terragrunt-atlantis-config/tree/master/test_examples/basic_module

	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  terragrunt: true
  terragrunt_parsing:
    parallel: true
    createProjectName: true
    createWorkspace: true
    defaultWorkflow: default

`
	hclFile := `terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

inputs = {
  foo = "bar"
}
`
	defer createFile(path.Join(tempDir, "digger.yml"), diggerCfg)()
	defer createFile(path.Join(tempDir, "terragrunt.hcl"), hclFile)()

	_, config, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err)

	print(config)
}

func TestDiggerTerragruntInfrastructureLiveExample(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
generate_projects:
  terragrunt: true
  terragrunt_parsing:
    parallel: true
    createProjectName: true
    createWorkspace: true
    defaultWorkflow: default
`

	repoUrl := "https://github.com/gruntwork-io/terragrunt-infrastructure-live-example"
	_, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})
	assert.NoError(t, err)

	defer createFile(path.Join(tempDir, "digger.yml"), diggerCfg)()

	_, config, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, config)

	assert.Equal(t, "non-prod_us-east-1_qa_mysql", config.Projects[0].Name)
	assert.Equal(t, "non-prod_us-east-1_qa_webserver-cluster", config.Projects[1].Name)
	assert.Equal(t, "non-prod_us-east-1_stage_mysql", config.Projects[2].Name)
	assert.Equal(t, "non-prod_us-east-1_stage_webserver-cluster", config.Projects[3].Name)
	assert.Equal(t, "prod_us-east-1_prod_mysql", config.Projects[4].Name)
	assert.Equal(t, "prod_us-east-1_prod_webserver-cluster", config.Projects[5].Name)
}

func TestDiggerGenerateProjectsMultipleBlocksDemo(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	repoUrl := "https://github.com/diggerhq/generate_projects_multiple_blocks_demo"
	_, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})
	assert.NoError(t, err)

	_, config, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "projects_dev_test1", config.Projects[0].Name)
	assert.Equal(t, "projects/dev/test1", config.Projects[0].Dir)
	assert.Equal(t, "projects_dev_test2", config.Projects[1].Name)
	assert.Equal(t, "projects/dev/test2", config.Projects[1].Dir)
	assert.Equal(t, "projects_dev_test3", config.Projects[2].Name)
	assert.Equal(t, "projects/dev/test3", config.Projects[2].Dir)
	assert.Equal(t, "projects_prod_test1", config.Projects[3].Name)
	assert.Equal(t, "projects/prod/test1", config.Projects[3].Dir)
	assert.Equal(t, "projects_prod_test2", config.Projects[4].Name)
	assert.Equal(t, "projects/prod/test2", config.Projects[4].Dir)
	assert.Equal(t, 5, len(config.Projects))
}

// todo test terragrunt digger_config with terragrunt_parsing block but without terragrunt: true

// TestDiggerTraverseToNestedProjects test if traverse_to_nested_projects is set to true, digger will traverse to nested projects
func TestDiggerTraverseToNestedProjects(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
allow_draft_prs: true
traverse_to_nested_projects: true
generate_projects:
  blocks:
    - include: dev/**
      aws_role_to_assume:
        state: "arn://abc:xyz:state"
        command: "arn://abc:xyz:cmd"
      aws_cognito_oidc:
        cognito_identity_pool_id: "us-east-1:00000000-0000-0000-0000-000000000000"
        aws_region: "us-east-1"
        aws_account_id: "000000000000"
`
	deleteFile := createFile(path.Join(tempDir, "digger.yml"), diggerCfg)
	defer deleteFile()
	dirsToCreate := []string{"dev/test1", "dev/test2", "dev/project", "dev/project/test3", "testtt"}

	for _, dir := range dirsToCreate {
		err := os.MkdirAll(path.Join(tempDir, dir), os.ModePerm)
		defer createFile(path.Join(tempDir, dir, "main.tf"), "")()
		assert.NoError(t, err, "expected error to be nil")
	}

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")
	assert.Equal(t, true, dg.TraverseToNestedProjects)
	assert.Equal(t, 4, len(dg.Projects))
	assert.Equal(t, "arn://abc:xyz:cmd", dg.Projects[0].AwsRoleToAssume.Command)
	assert.Equal(t, "arn://abc:xyz:state", dg.Projects[0].AwsRoleToAssume.State)
	assert.Equal(t, "us-east-1:00000000-0000-0000-0000-000000000000", dg.Projects[0].AwsCognitoOidcConfig.CognitoPoolId)
	assert.Equal(t, "us-east-1", dg.Projects[0].AwsCognitoOidcConfig.AwsRegion)
	assert.Equal(t, "000000000000", dg.Projects[0].AwsCognitoOidcConfig.AwsAccountId)
	assert.Equal(t, "dev_project", dg.Projects[0].Name)
	assert.Equal(t, "dev/project", dg.Projects[0].Dir)
	assert.Equal(t, "dev_project_test3", dg.Projects[1].Name)
	assert.Equal(t, "dev/project/test3", dg.Projects[1].Dir)
	assert.Equal(t, "dev_test1", dg.Projects[2].Name)
	assert.Equal(t, "dev/test1", dg.Projects[2].Dir)
	assert.Equal(t, "dev_test2", dg.Projects[3].Name)
	assert.Equal(t, "dev/test2", dg.Projects[3].Dir)
	assert.Equal(t, true, dg.AllowDraftPRs)
}

// TestDiggerAllowDraftPRs tests if allow_draft_prs is set to true, digger will allow draft PRs. Defaults to false
func TestDiggerAllowDraftPRs(t *testing.T) {
	tempDir, teardown := setUp()
	defer teardown()

	diggerCfg := `
projects:
- name: dev
  dir: .
`
	defer createFile(path.Join(tempDir, "digger.yml"), diggerCfg)()
	defer createFile(path.Join(tempDir, "main.tf"), "resource \"null_resource\" \"test4\" {}")()

	dg, _, _, err := LoadDiggerConfig(tempDir, true, nil)
	assert.NoError(t, err)
	assert.Equal(t, false, dg.AllowDraftPRs)
}

func TestGetModifiedProjectsReturnsCorrectSourceMapping(t *testing.T) {
	changedFiles := []string{"modules/bucket/main.tf", "dev/main.tf"}
	projects := []Project{
		Project{
			Name:            "dev",
			Dir:             "dev",
			IncludePatterns: []string{"modules/**"},
		},
		Project{
			Name:            "prod",
			Dir:             "prod",
			IncludePatterns: []string{"modules/**"},
		},
	}
	c := DiggerConfig{
		Projects: projects,
	}
	expectedImpactingLocations := map[string]ProjectToSourceMapping{
		"dev":  {ImpactingLocations: []string{"modules/bucket", "dev"}},
		"prod": {ImpactingLocations: []string{"modules/bucket"}},
	}

	impactedProjects, projectSourceMapping := c.GetModifiedProjects(changedFiles)
	assert.Equal(t, 2, len(impactedProjects))
	assert.Equal(t, 2, len(projectSourceMapping))
	assert.Equal(t, 2, len(projectSourceMapping["dev"].ImpactingLocations))
	assert.Equal(t, 1, len(projectSourceMapping["prod"].ImpactingLocations))
	assert.Equal(t, expectedImpactingLocations["dev"].ImpactingLocations, projectSourceMapping["dev"].ImpactingLocations)
	assert.Equal(t, expectedImpactingLocations["prod"].ImpactingLocations, projectSourceMapping["prod"].ImpactingLocations)

}

func TestCognitoTokenSetFromMinConfig(t *testing.T) {
	diggerCfg := `
projects:
  - name: dev
    dir: .
    aws_cognito_oidc:
      cognito_identity_pool_id: "us-east-1:00000000-0000-0000-0000-000000000000"
      aws_account_id: "000000000000"
    aws_role_to_assume:
      state: "arn://abc:xyz:state"
      command: "arn://abc:xyz:cmd"
`
	dg, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	assert.NoError(t, err, "expected error to be nil")
	assert.NotNil(t, dg, "expected digger digger_config to be not nil")

	assert.Equal(t, dg.Projects[0].AwsCognitoOidcConfig.CognitoPoolId, "us-east-1:00000000-0000-0000-0000-000000000000")
	assert.Equal(t, dg.Projects[0].AwsCognitoOidcConfig.AwsAccountId, "000000000000")
}
