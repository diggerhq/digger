package atlantis

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/singleflight"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
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

	// reset caches
	getDependenciesCache = newGetDependenciesCache()
	requestGroup = singleflight.Group{}
	return nil
}

func runTest(t *testing.T, goldenFile string, testPath string, createProjectName bool, workflowName string, withWorkspace bool, parallel bool, ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, cascadeDependencies bool) {
	resetForRun()
	atlantisConfig, _, err := Parse(
		testPath,
		nil,
		true,
		false,
		parallel,
		"",
		false,
		ignoreParentTerragrunt,
		ignoreDependencyBlocks,
		true,
		workflowName,
		nil,
		false,
		"",
		createProjectName,
		withWorkspace,
		true,
		false,
		false,
		false,
	)

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
	runTest(t, "golden/basic.yaml", "test_examples/basic_module", false, "", false, true, true, false, false)
}

func TestBasicModuleWithWorkspace(t *testing.T) {
	runTest(t, "golden/withWorkspace.yaml", "test_examples/basic_module", false, "", true, true, true, false, false)
}

func TestBasicModuleWithWorkflowSpecified(t *testing.T) {
	runTest(t, "golden/namedWorkflow.yaml", "test_examples/basic_module", false, "someWorkflow", false, true, true, false, false)
}

func TestBasicModuleWithParallelDisabled(t *testing.T) {
	runTest(t, "golden/noParallel.yaml", "test_examples/basic_module", false, "", false, false, true, false, false)
}

func TestChainedDependencies(t *testing.T) {
	runTest(t, "golden/chained_dependency.yaml", "test_examples/chained_dependencies", false, "", false, true, true, false, false)
}

func TestInvalidParentModule(t *testing.T) {
	runTest(t, "golden/invalid_parent_module.yaml", "test_examples/invalid_parent_module", false, "", false, true, true, false, false)
}

func TestParentAndChildDefinedWorkflow(t *testing.T) {
	runTest(t, "golden/parentAndChildDefinedWorkflow.yaml", "test_examples/child_and_parent_specify_workflow", false, "", false, true, true, false, false)
}

func TestParentDefinedWorkflow(t *testing.T) {
	runTest(t, "golden/parentDefinedWorkflow.yaml", "test_examples/parent_with_workflow_local", false, "", false, true, true, false, false)
}

func TestIgnoringParentTerragrunt(t *testing.T) {
	runTest(t, "golden/withoutParent.yaml", "test_examples/with_parent", false, "", false, true, true, false, false)
}

// TODO: figure out why this test is succeeding locally but failing in CI
// it might be a race condition of resetting the variables but I haven't yet figured it out
func TestNotIgnoringParentTerragrunt(t *testing.T) {
	t.Skip()
	runTest(t, "golden/withParent.yaml", "test_examples/with_parent", false, "", false, true, false, false, false)
}

func TestTerragruntDependencies(t *testing.T) {
	runTest(t, "golden/terragrunt_dependency.yaml", "test_examples/terragrunt_dependency", false, "", false, true, true, false, false)
}

func TestIgnoringTerragruntDependencies(t *testing.T) {
	runTest(t, "golden/terragrunt_dependency_ignored.yaml", "test_examples/terragrunt_dependency", false, "", false, true, true, true, false)
}

func TestUnparseableParent(t *testing.T) {
	runTest(t, "golden/invalid_parent_module.yaml", "test_examples/invalid_parent_module", false, "", false, true, true, false, false)
}

func TestWithProjectNames(t *testing.T) {
	runTest(t, "golden/withProjectName.yaml", "test_examples/invalid_parent_module", true, "", false, true, true, false, false)
}

// TODO: fix this test
func TestMergingLocalDependenciesFromParent(t *testing.T) {
	t.Skip()
	runTest(t, "golden/mergeParentDependencies.yaml", "test_examples/parent_with_extra_deps", false, "", false, true, true, false, false)
}

func TestInfrastructureLive(t *testing.T) {
	runTest(t, "golden/infrastructureLive.yaml", "test_examples/terragrunt-infrastructure-live-example", false, "", false, true, true, false, false)
}

func TestModulesWithNoTerraformSourceDefinitions(t *testing.T) {
	runTest(t, "golden/no_terraform_blocks.yml", "test_examples/no_terraform_blocks", false, "", false, true, true, false, false)
}

func TestInfrastructureMutliAccountsVPCRoute53TGWCascading(t *testing.T) {
	runTest(t, "golden/multi_accounts_vpc_route53_tgw.yaml", "test_examples/multi_accounts_vpc_route53_tgw", false, "", false, true, true, false, true)
}
