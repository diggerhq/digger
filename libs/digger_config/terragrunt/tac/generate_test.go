package tac

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	var level slog.Leveler
	level = slog.LevelDebug
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func resetForRun() error {
	return nil
}

func runTest(t *testing.T, goldenFile string, testPath string, createProjectName bool, workflowName string, withWorkspace bool, parallel bool, ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, cascadeDependencies bool, aliasDelimiter string) {
	resetForRun()
	atlantisConfig, _, err := Parse(testPath, nil, true, false, parallel, make([]string, 0), false, ignoreParentTerragrunt, ignoreDependencyBlocks, true, workflowName, nil, false, "", aliasDelimiter, createProjectName, withWorkspace, true, false, false, false, nil)

	if err != nil {
		slog.Error("failed to parse terragrunt configuration", "error", err)
		t.Fatal(err)
	}

	var contents []byte
	yaml.Unmarshal(contents, &atlantisConfig)

	goldenContentsBytes, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Error("Failed to read golden file")
		return
	}

	goldenContents := &AtlantisConfig{}
	err = yaml.Unmarshal(goldenContentsBytes, goldenContents)
	if err != nil {
		t.Error("error unmarshalling golden file")
		return
	}

	assert.Equal(t, goldenContents, atlantisConfig)
}

func TestBasicModule(t *testing.T) {
	runTest(t, "golden/basic.yaml", "test_examples/basic_module", false, "", false, true, true, false, false, "_")
}

func TestBasicModuleWithWorkspace(t *testing.T) {
	runTest(t, "golden/withWorkspace.yaml", "test_examples/basic_module", false, "", true, true, true, false, false, "_")
}

func TestBasicModuleWithWorkflowSpecified(t *testing.T) {
	runTest(t, "golden/namedWorkflow.yaml", "test_examples/basic_module", false, "someWorkflow", false, true, true, false, false, "_")
}

func TestBasicModuleWithParallelDisabled(t *testing.T) {
	runTest(t, "golden/noParallel.yaml", "test_examples/basic_module", false, "", false, false, true, false, false, "_")
}

func TestChainedDependencies(t *testing.T) {
	runTest(t, "golden/chained_dependency.yaml", "test_examples/chained_dependencies", false, "", false, true, true, false, false, "_")
}

func TestInvalidParentModule(t *testing.T) {
	runTest(t, "golden/invalid_parent_module.yaml", "test_examples/invalid_parent_module", false, "", false, true, true, false, false, "_")
}

func TestParentAndChildDefinedWorkflow(t *testing.T) {
	runTest(t, "golden/parentAndChildDefinedWorkflow.yaml", "test_examples/child_and_parent_specify_workflow", false, "", false, true, true, false, false, "_")
}

func TestParentDefinedWorkflow(t *testing.T) {
	runTest(t, "golden/parentDefinedWorkflow.yaml", "test_examples/parent_with_workflow_local", false, "", false, true, true, false, false, "_")
}

func TestIgnoringParentTerragrunt(t *testing.T) {
	runTest(t, "golden/withoutParent.yaml", "test_examples/with_parent", false, "", false, true, true, false, false, "_")
}

// TODO: figure out why this test is succeeding locally but failing in CI
// it might be a race condition of resetting the variables but I haven't yet figured it out
func TestNotIgnoringParentTerragrunt(t *testing.T) {
	t.Skip()
	runTest(t, "golden/withParent.yaml", "test_examples/with_parent", false, "", false, true, false, false, false, "_")
}

func TestTerragruntDependencies(t *testing.T) {
	runTest(t, "golden/terragrunt_dependency.yaml", "test_examples/terragrunt_dependency", false, "", false, true, true, false, false, "_")
}

func TestIgnoringTerragruntDependencies(t *testing.T) {
	runTest(t, "golden/terragrunt_dependency_ignored.yaml", "test_examples/terragrunt_dependency", false, "", false, true, true, true, false, "_")
}

func TestUnparseableParent(t *testing.T) {
	runTest(t, "golden/invalid_parent_module.yaml", "test_examples/invalid_parent_module", false, "", false, true, true, false, false, "_")
}

func TestWithProjectNames(t *testing.T) {
	runTest(t, "golden/withProjectName.yaml", "test_examples/invalid_parent_module", true, "", false, true, true, false, false, "_")
}

func TestWithProjectNamesAndAliasDelimiter(t *testing.T) {
	runTest(t, "golden/withProjectNameAndAliasDelimiter.yaml", "test_examples/invalid_parent_module", true, "", false, true, true, false, false, "/")
}

// TODO: fix this test
func TestMergingLocalDependenciesFromParent(t *testing.T) {
	t.Skip()
	runTest(t, "golden/mergeParentDependencies.yaml", "test_examples/parent_with_extra_deps", false, "", false, true, true, false, false, "_")
}

func TestInfrastructureLive(t *testing.T) {
	runTest(t, "golden/infrastructureLive.yaml", "test_examples/terragrunt-infrastructure-live-example", false, "", false, true, true, false, false, "_")
}

func TestModulesWithNoTerraformSourceDefinitions(t *testing.T) {
	runTest(t, "golden/no_terraform_blocks.yml", "test_examples/no_terraform_blocks", false, "", false, true, true, false, false, "_")
}

func TestInfrastructureMutliAccountsVPCRoute53TGWCascading(t *testing.T) {
	runTest(t, "golden/multi_accounts_vpc_route53_tgw.yaml", "test_examples/multi_accounts_vpc_route53_tgw", false, "", false, true, true, false, true, "_")
}

func TestInferProjectWhenModifiedPatternsDoesNotExecuteRunCmd(t *testing.T) {
	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "run-cmd-executed")

	terragruntContents := fmt.Sprintf(`
locals {
  touched = run_cmd("sh", "-c", "touch %s")
}

terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}
`, markerPath)

	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "terragrunt.hcl"), []byte(terragruntContents), 0o644))

	patterns, err := InferProjectWhenModifiedPatterns(tempDir, ".", false, false, true, false)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"*.hcl", "*.tf*"}, patterns)
	_, statErr := os.Stat(markerPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestInferProjectWhenModifiedPatternsDoesNotReuseParentSkipAcrossCalls(t *testing.T) {
	tempDir := t.TempDir()

	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "terragrunt.hcl"), []byte(`
locals {
  ahhhhhh = "pst"
}
`), 0o644))

	patterns, err := InferProjectWhenModifiedPatterns(tempDir, ".", true, false, true, false)
	assert.NoError(t, err)
	assert.Nil(t, patterns)

	patterns, err = InferProjectWhenModifiedPatterns(tempDir, ".", false, false, true, false)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"*.hcl", "*.tf*"}, patterns)
}

func TestInferProjectWhenModifiedPatternsDoesNotSkipIncludedParentAfterChild(t *testing.T) {
	tempDir := t.TempDir()
	childDir := filepath.Join(tempDir, "child")

	assert.NoError(t, os.MkdirAll(childDir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "terragrunt.hcl"), []byte(`
locals {
  parent = "root"
}
`), 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(childDir, "terragrunt.hcl"), []byte(`
include {
  path = find_in_parent_folders()
}

terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}
`), 0o644))

	childPatterns, err := InferProjectWhenModifiedPatterns(tempDir, "child", false, false, true, false)
	assert.NoError(t, err)
	assert.NotNil(t, childPatterns)

	parentPatterns, err := InferProjectWhenModifiedPatterns(tempDir, ".", false, false, true, false)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"*.hcl", "*.tf*"}, parentPatterns)
}
