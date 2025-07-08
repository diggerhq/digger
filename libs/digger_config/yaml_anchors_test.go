package digger_config

import (
	"os"
	"testing"
)

func TestYamlAnchorsCurrentBehavior(t *testing.T) {
	diggerCfg := `
# Define common workflow steps as anchors
common_steps: &common_steps
  steps:
    - init
    - plan

apply_steps: &apply_steps
  steps:
    - init  
    - apply

projects:
- name: dev
  dir: dev
  workflow: dev_workflow

workflows:
  dev_workflow:
    plan: *common_steps
    apply: *apply_steps
`

	config, configYaml, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	
	// Let's see what happens currently
	t.Logf("Error: %v", err)
	t.Logf("Config: %+v", config)
	t.Logf("ConfigYaml: %+v", configYaml)
	
	if err != nil {
		t.Logf("Current implementation doesn't support YAML anchors: %v", err)
	} else {
		t.Logf("Current implementation supports YAML anchors!")
		// Check if the workflow steps were resolved correctly
		if config != nil && config.Workflows != nil {
			workflow, exists := config.Workflows["dev_workflow"]
			if exists {
				t.Logf("Plan steps: %+v", workflow.Plan.Steps)
				t.Logf("Apply steps: %+v", workflow.Apply.Steps)
			}
		}
	}
}

func TestYamlAnchorsWithEnvVars(t *testing.T) {
	diggerCfg := `
# Define common env vars as anchors
common_env_vars: &common_env_vars
  state:
    - name: TF_VAR_region
      value: us-east-1
  commands:
    - name: TF_LOG
      value: INFO

projects:
- name: dev
  dir: dev
  workflow: dev_workflow
- name: prod
  dir: prod  
  workflow: prod_workflow

workflows:
  dev_workflow:
    env_vars: *common_env_vars
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - init
        - apply
  prod_workflow:
    env_vars: *common_env_vars
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - init
        - apply
`

	config, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	
	t.Logf("Error: %v", err)
	if err != nil {
		t.Logf("Current implementation doesn't support YAML anchors with env vars: %v", err)
	} else {
		t.Logf("Current implementation supports YAML anchors with env vars!")
		if config != nil && config.Workflows != nil {
			devWorkflow, devExists := config.Workflows["dev_workflow"]
			prodWorkflow, prodExists := config.Workflows["prod_workflow"]
			if devExists && prodExists {
				t.Logf("Dev workflow env vars: %+v", devWorkflow.EnvVars)
				t.Logf("Prod workflow env vars: %+v", prodWorkflow.EnvVars)
			}
		}
	}
}

func TestYamlAnchorsWithProjectConfig(t *testing.T) {
	diggerCfg := `
# Define common project configuration
common_aws_config: &common_aws_config
  aws_role_to_assume:
    state: "arn:aws:iam::123456789012:role/digger-state"
    command: "arn:aws:iam::123456789012:role/digger-command"

projects:
- name: dev
  dir: dev
  <<: *common_aws_config
  workflow: default
- name: prod
  dir: prod
  <<: *common_aws_config
  workflow: default
`

	config, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if config == nil || len(config.Projects) != 2 {
		t.Fatalf("Expected 2 projects, got: %d", len(config.Projects))
	}
	
	for _, project := range config.Projects {
		if project.AwsRoleToAssume == nil {
			t.Errorf("Project %s should have AWS role configuration", project.Name)
		} else {
			if project.AwsRoleToAssume.State != "arn:aws:iam::123456789012:role/digger-state" {
				t.Errorf("Project %s has wrong state role: %s", project.Name, project.AwsRoleToAssume.State)
			}
			if project.AwsRoleToAssume.Command != "arn:aws:iam::123456789012:role/digger-command" {
				t.Errorf("Project %s has wrong command role: %s", project.Name, project.AwsRoleToAssume.Command)
			}
		}
	}
}

func TestYamlAnchorsComplexWorkflows(t *testing.T) {
	diggerCfg := `
# Define reusable workflow components
common_init: &common_init
  init:
    extra_args: ["-backend=true"]

common_plan: &common_plan
  plan:
    extra_args: ["-input=false", "-detailed-exitcode"]

common_apply: &common_apply
  apply:
    extra_args: ["-auto-approve"]

# Define full workflow templates
standard_plan: &standard_plan
  steps:
    - *common_init
    - *common_plan

standard_apply: &standard_apply
  steps:
    - *common_init
    - *common_apply

projects:
- name: app1
  dir: app1
  workflow: standard
- name: app2  
  dir: app2
  workflow: standard

workflows:
  standard:
    plan: *standard_plan
    apply: *standard_apply
`

	config, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if config == nil || len(config.Projects) != 2 {
		t.Fatalf("Expected 2 projects, got: %d", len(config.Projects))
	}
	
	workflow, exists := config.Workflows["standard"]
	if !exists {
		t.Fatal("Standard workflow should exist")
	}
	
	// Check plan steps
	if len(workflow.Plan.Steps) != 2 {
		t.Fatalf("Expected 2 plan steps, got: %d", len(workflow.Plan.Steps))
	}
	
	if workflow.Plan.Steps[0].Action != "init" {
		t.Errorf("First plan step should be init, got: %s", workflow.Plan.Steps[0].Action)
	}
	
	if len(workflow.Plan.Steps[0].ExtraArgs) != 1 || workflow.Plan.Steps[0].ExtraArgs[0] != "-backend=true" {
		t.Errorf("First plan step should have -backend=true extra arg, got: %v", workflow.Plan.Steps[0].ExtraArgs)
	}
	
	if workflow.Plan.Steps[1].Action != "plan" {
		t.Errorf("Second plan step should be plan, got: %s", workflow.Plan.Steps[1].Action)
	}
	
	if len(workflow.Plan.Steps[1].ExtraArgs) != 2 {
		t.Errorf("Second plan step should have 2 extra args, got: %v", workflow.Plan.Steps[1].ExtraArgs)
	}
	
	// Check apply steps
	if len(workflow.Apply.Steps) != 2 {
		t.Fatalf("Expected 2 apply steps, got: %d", len(workflow.Apply.Steps))
	}
	
	if workflow.Apply.Steps[0].Action != "init" {
		t.Errorf("First apply step should be init, got: %s", workflow.Apply.Steps[0].Action)
	}
	
	if workflow.Apply.Steps[1].Action != "apply" {
		t.Errorf("Second apply step should be apply, got: %s", workflow.Apply.Steps[1].Action)
	}
	
	if len(workflow.Apply.Steps[1].ExtraArgs) != 1 || workflow.Apply.Steps[1].ExtraArgs[0] != "-auto-approve" {
		t.Errorf("Second apply step should have -auto-approve extra arg, got: %v", workflow.Apply.Steps[1].ExtraArgs)
	}
}

func TestYamlAnchorsWithGenerateProjects(t *testing.T) {
	diggerCfg := `
# Define common AWS configuration as anchor
aws_prod_config: &aws_prod_config
  aws_role_to_assume:
    state: "arn:aws:iam::123456789012:role/digger-prod-state"
    command: "arn:aws:iam::123456789012:role/digger-prod-command"

aws_dev_config: &aws_dev_config  
  aws_role_to_assume:
    state: "arn:aws:iam::123456789012:role/digger-dev-state"
    command: "arn:aws:iam::123456789012:role/digger-dev-command"

generate_projects:
  blocks:
    - include: dev/*
      <<: *aws_dev_config
      workflow: dev_workflow
    - include: prod/*
      <<: *aws_prod_config
      workflow: prod_workflow

workflows:
  dev_workflow:
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - init
        - apply
  prod_workflow:
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - init
        - apply
`

	config, _, _, err := LoadDiggerConfigFromString(diggerCfg, "./")
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Since no actual directories exist in the test, we should have 0 projects
	// But the configuration should parse correctly
	if config == nil {
		t.Fatal("Config should not be nil")
	}
	
	// Verify workflows exist
	if _, exists := config.Workflows["dev_workflow"]; !exists {
		t.Error("dev_workflow should exist")
	}
	
	if _, exists := config.Workflows["prod_workflow"]; !exists {
		t.Error("prod_workflow should exist")
	}
}

func TestYamlAnchorsExampleConfig(t *testing.T) {
	// Test the example configuration file we created
	exampleContent, err := os.ReadFile("examples/digger-with-anchors.yml")
	if err != nil {
		t.Skipf("Example file not found: %v", err)
	}
	
	config, _, _, err := LoadDiggerConfigFromString(string(exampleContent), "./")
	
	if err != nil {
		t.Fatalf("Expected no error loading example config, got: %v", err)
	}
	
	if config == nil {
		t.Fatal("Config should not be nil")
	}
	
	// Verify we have 4 projects
	if len(config.Projects) != 4 {
		t.Fatalf("Expected 4 projects, got: %d", len(config.Projects))
	}
	
	// Verify project names
	expectedProjects := []string{"webapp-dev", "webapp-prod", "database-dev", "database-prod"}
	for i, project := range config.Projects {
		if project.Name != expectedProjects[i] {
			t.Errorf("Expected project %d to be %s, got %s", i, expectedProjects[i], project.Name)
		}
	}
	
	// Verify AWS roles are properly set
	devProjects := []string{"webapp-dev", "database-dev"}
	prodProjects := []string{"webapp-prod", "database-prod"}
	
	for _, project := range config.Projects {
		if contains(devProjects, project.Name) {
			if project.AwsRoleToAssume == nil {
				t.Errorf("Project %s should have AWS role configuration", project.Name)
			} else {
				if project.AwsRoleToAssume.State != "arn:aws:iam::123456789012:role/digger-dev-state" {
					t.Errorf("Project %s has wrong dev state role", project.Name)
				}
			}
		} else if contains(prodProjects, project.Name) {
			if project.AwsRoleToAssume == nil {
				t.Errorf("Project %s should have AWS role configuration", project.Name)
			} else {
				if project.AwsRoleToAssume.State != "arn:aws:iam::987654321098:role/digger-prod-state" {
					t.Errorf("Project %s has wrong prod state role", project.Name)
				}
			}
		}
	}
	
	// Verify workflows exist and have correct configuration
	devWorkflow, devExists := config.Workflows["development"]
	if !devExists {
		t.Error("development workflow should exist")
	} else {
		if len(devWorkflow.EnvVars.State) != 2 {
			t.Errorf("development workflow should have 2 state env vars, got %d", len(devWorkflow.EnvVars.State))
		}
	}
	
	prodWorkflow, prodExists := config.Workflows["production"]
	if !prodExists {
		t.Error("production workflow should exist")
	} else {
		// Production workflow should have 3 apply steps (init, run, apply)
		if len(prodWorkflow.Apply.Steps) != 3 {
			t.Errorf("production workflow should have 3 apply steps, got %d", len(prodWorkflow.Apply.Steps))
		}
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}