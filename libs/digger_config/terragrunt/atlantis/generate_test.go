package atlantis

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/singleflight"
)

func RunWithFlags(filename string, args []string) ([]byte, error) {
	rootCmd.SetArgs(args)
	rootCmd.Execute()

	return ioutil.ReadFile(filename)
}

// Resets all flag values to their defaults in between tests
func resetForRun() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// reset caches
	getDependenciesCache = newGetDependenciesCache()
	requestGroup = singleflight.Group{}
	// reset flags
	gitRoot = pwd
	autoPlan = false
	autoMerge = false
	cascadeDependencies = true
	ignoreParentTerragrunt = true
	ignoreDependencyBlocks = false
	parallel = true
	createWorkspace = false
	createProjectName = false
	preserveWorkflows = true
	preserveProjects = true
	defaultWorkflow = ""
	filterPath = ""
	outputPath = ""
	defaultTerraformVersion = ""
	defaultApplyRequirements = []string{}
	projectHclFiles = []string{}
	createHclProjectChilds = false
	createHclProjectExternalChilds = true
	useProjectMarkers = false

	return nil
}

// Runs a test, asserting the output produced matches a golden file
func runTest(t *testing.T, goldenFile string, args []string) {
	err := resetForRun()
	if err != nil {
		t.Error("Failed to reset default flags")
		return
	}

	randomInt := rand.Int()
	filename := filepath.Join("test_artifacts", fmt.Sprintf("%d.yaml", randomInt))
	defer os.Remove(filename)

	allArgs := append([]string{
		"generate",
		"--output",
		filename,
	}, args...)

	contentBytes, err := RunWithFlags(filename, allArgs)
	content := &AtlantisConfig{}
	yaml.Unmarshal(contentBytes, content)
	if err != nil {
		t.Error(err)
		return
	}

	goldenContentsBytes, err := ioutil.ReadFile(goldenFile)
	goldenContents := &AtlantisConfig{}
	yaml.Unmarshal(goldenContentsBytes, goldenContents)
	if err != nil {
		t.Error("Failed to read golden file")
		return
	}

	assert.Equal(t, goldenContents, content)
}

func TestSettingRoot(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
	})
}

func TestRootPathBeingAbsolute(t *testing.T) {
	parent, err := filepath.Abs(filepath.Join("..", "test_examples", "basic_module"))
	if err != nil {
		t.Error("Failed to find parent directory")
	}

	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		parent,
	})
}

func TestRootPathHavingTrailingSlash(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module") + string(filepath.Separator),
	})
}

func TestWithNoTerragruntFiles(t *testing.T) {
	runTest(t, filepath.Join("golden", "empty.yaml"), []string{
		"--root",
		".", // There are no terragrunt files in this directory
		filepath.Join("..", "test_examples", "no_modules"),
	})
}

func TestWithParallelizationDisabled(t *testing.T) {
	runTest(t, filepath.Join("golden", "noParallel.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
		"--parallel=false",
	})
}

func TestIgnoringParentTerragrunt(t *testing.T) {
	runTest(t, filepath.Join("golden", "withoutParent.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "with_parent"),
	})
}

func TestNotIgnoringParentTerragrunt(t *testing.T) {
	runTest(t, filepath.Join("golden", "withParent.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "with_parent"),
		"--ignore-parent-terragrunt=false",
	})
}

func TestEnablingAutoplan(t *testing.T) {
	runTest(t, filepath.Join("golden", "withAutoplan.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
		"--autoplan",
	})
}

func TestSettingWorkflowName(t *testing.T) {
	runTest(t, filepath.Join("golden", "namedWorkflow.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
		"--workflow",
		"someWorkflow",
	})
}

func TestExtraDeclaredDependencies(t *testing.T) {
	runTest(t, filepath.Join("golden", "extra_dependencies.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "extra_dependency"),
	})
}

func TestLocalTerraformModuleSource(t *testing.T) {
	runTest(t, filepath.Join("golden", "local_terraform_module.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "local_terraform_module_source"),
	})
}

func TestLocalTerraformAbsModuleSource(t *testing.T) {
	runTest(t, filepath.Join("golden", "local_terraform_abs_module.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "local_terraform_abs_module_source"),
	})
}

func TestLocalTfModuleSource(t *testing.T) {
	runTest(t, filepath.Join("golden", "local_tf_module.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "local_tf_module_source"),
	})
}

func TestTerragruntDependencies(t *testing.T) {
	runTest(t, filepath.Join("golden", "terragrunt_dependency.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terragrunt_dependency"),
	})
}

func TestIgnoringTerragruntDependencies(t *testing.T) {
	runTest(t, filepath.Join("golden", "terragrunt_dependency_ignored.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terragrunt_dependency"),
		"--ignore-dependency-blocks",
	})
}

func TestCustomWorkflowName(t *testing.T) {
	runTest(t, filepath.Join("golden", "different_workflow_names.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "different_workflow_names"),
	})
}

// This test covers parent Terragrunt files that are not runnable as modules themselves.
// Sometimes it is possible to have parent files that only are runnable when included
// into child modules.
func TestUnparseableParent(t *testing.T) {
	runTest(t, filepath.Join("golden", "invalid_parent_module.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "invalid_parent_module"),
	})
}

func TestWithWorkspaces(t *testing.T) {
	runTest(t, filepath.Join("golden", "withWorkspace.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
		"--create-workspace",
	})
}

func TestWithProjectNames(t *testing.T) {
	runTest(t, filepath.Join("golden", "withProjectName.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "invalid_parent_module"),
		"--create-project-name",
	})
}

func TestMergingLocalDependenciesFromParent(t *testing.T) {
	runTest(t, filepath.Join("golden", "mergeParentDependencies.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "parent_with_extra_deps"),
	})
}

func TestWorkflowFromParentInLocals(t *testing.T) {
	runTest(t, filepath.Join("golden", "parentDefinedWorkflow.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "parent_with_workflow_local"),
	})
}

func TestChildWorkflowOverridesParentWorkflow(t *testing.T) {
	runTest(t, filepath.Join("golden", "parentAndChildDefinedWorkflow.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "child_and_parent_specify_workflow"),
	})
}

func TestExtraArguments(t *testing.T) {
	runTest(t, filepath.Join("golden", "extraArguments.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "extra_arguments"),
	})
}

func TestInfrastructureLive(t *testing.T) {
	runTest(t, filepath.Join("golden", "infrastructureLive.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example"),
	})
}

func TestModulesWithNoTerraformSourceDefinitions(t *testing.T) {
	runTest(t, filepath.Join("golden", "no_terraform_blocks.yml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "no_terraform_blocks"),
		"--parallel",
		"--autoplan",
	})
}

func TestInfrastructureMutliAccountsVPCRoute53TGWCascading(t *testing.T) {
	runTest(t, filepath.Join("golden", "multi_accounts_vpc_route53_tgw.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "multi_accounts_vpc_route53_tgw"),
		"--cascade-dependencies",
	})
}

func TestAutoPlan(t *testing.T) {
	runTest(t, filepath.Join("golden", "autoplan.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "autoplan"),
		"--autoplan=false",
	})
}

func TestSkippingModules(t *testing.T) {
	runTest(t, filepath.Join("golden", "skip.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "skip"),
	})
}

func TestTerraformVersionConfig(t *testing.T) {
	runTest(t, filepath.Join("golden", "terraform_version.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terraform_version"),
		"--terraform-version", "0.14.9001",
	})
}

func TestPreservingOldProjects(t *testing.T) {
	err := resetForRun()
	if err != nil {
		t.Error("Failed to reset default flags")
		return
	}

	randomInt := rand.Int()
	filename := filepath.Join("test_artifacts", fmt.Sprintf("%d.yaml", randomInt))
	defer os.Remove(filename)

	// Create an existing file to simulate an existing atlantis.yaml file
	contents := []byte(`projects:
- autoplan:
    enabled: false
    when_modified:
    - '*.hcl'
    - '*.tf*'
  dir: someDir
  name: projectFromPreviousRun 
`)
	ioutil.WriteFile(filename, contents, 0644)

	content, err := RunWithFlags(filename, []string{
		"generate",
		"--preserve-projects",
		"--output",
		filename,
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
	})
	if err != nil {
		t.Error("Failed to read file")
		return
	}

	goldenContents, err := ioutil.ReadFile(filepath.Join("golden", "oldProjectsPreserved.yaml"))
	if err != nil {
		t.Error("Failed to read golden file")
		return
	}

	if string(content) != string(goldenContents) {
		t.Errorf("Content did not match golden file.\n\nExpected Content: %s\n\nContent: %s", string(goldenContents), string(content))
	}
}

func TestEnablingAutomerge(t *testing.T) {
	runTest(t, filepath.Join("golden", "withAutomerge.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
		"--automerge",
	})
}

func TestChainedDependencies(t *testing.T) {
	runTest(t, filepath.Join("golden", "chained_dependency.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "chained_dependencies"),
		"--cascade-dependencies",
	})
}

func TestChainedDependenciesHiddenBehindFlag(t *testing.T) {
	runTest(t, filepath.Join("golden", "chained_dependency_no_flag.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "chained_dependencies"),
		"--cascade-dependencies=false",
	})
}

func TestApplyRequirementsLocals(t *testing.T) {
	runTest(t, filepath.Join("golden", "apply_overrides.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "apply_requirements_overrides"),
	})
}

func TestApplyRequirementsFlag(t *testing.T) {
	runTest(t, filepath.Join("golden", "apply_overrides_flag.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "basic_module"),
		"--apply-requirements=approved,mergeable",
	})
}

func TestFilterFlagWithInfraLiveProd(t *testing.T) {
	runTest(t, filepath.Join("golden", "filterInfraLiveProd.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example"),
		"--filter",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example", "prod"),
	})
}

func TestFilterFlagWithInfraLiveNonProd(t *testing.T) {
	runTest(t, filepath.Join("golden", "filterInfraLiveNonProd.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example"),
		"--filter",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example", "non-prod"),
	})
}

func TestFilterGlobFlagWithInfraLiveMySql(t *testing.T) {
	runTest(t, filepath.Join("golden", "filterGlobInfraLiveMySQL.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example"),
		"--filter",
		filepath.Join("..", "test_examples", "terragrunt-infrastructure-live-example", "*", "*", "*", "mysql"),
	})
}

func TestMultipleIncludes(t *testing.T) {
	runTest(t, filepath.Join("golden", "multiple_includes.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "multiple_includes"),
		"--terraform-version", "0.14.9001",
	})
}

func TestRemoteModuleSourceBitbucket(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_bitbucket"),
	})
}

func TestRemoteModuleSourceGCS(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_gcs"),
	})
}

func TestRemoteModuleSourceGitHTTPS(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_git_https"),
	})
}

func TestRemoteModuleSourceGitSCPLike(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_git_scp_like"),
	})
}

func TestRemoteModuleSourceGitSSH(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_git_ssh"),
	})
}

func TestRemoteModuleSourceGithubHTTPS(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_github_https"),
	})
}

func TestRemoteModuleSourceGithubSSH(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_github_ssh"),
	})
}

func TestRemoteModuleSourceHTTP(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_http"),
	})
}

func TestRemoteModuleSourceHTTPS(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_https"),
	})
}

func TestRemoteModuleSourceMercurial(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_mercurial"),
	})
}

func TestRemoteModuleSourceS3(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_s3"),
	})
}

func TestRemoteModuleSourceTerraformRegistry(t *testing.T) {
	runTest(t, filepath.Join("golden", "basic.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "remote_module_source_terraform_registry"),
	})
}

func TestEnvHCLProjectsNoChilds(t *testing.T) {
	runTest(t, filepath.Join("golden", "envhcl_nochilds.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples"),
		"--project-hcl-files=env.hcl",
		"--create-hcl-project-childs=false",
		"--create-hcl-project-external-childs=false",
	})
}

func TestEnvHCLProjectsSubChilds(t *testing.T) {
	runTest(t, filepath.Join("golden", "envhcl_subchilds.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples"),
		"--project-hcl-files=env.hcl",
		"--create-hcl-project-childs=true",
		"--create-hcl-project-external-childs=false",
	})
}

func TestEnvHCLProjectsExternalChilds(t *testing.T) {
	runTest(t, filepath.Join("golden", "envhcl_externalchilds.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples"),
		"--project-hcl-files=env.hcl",
		"--create-hcl-project-childs=false",
		"--create-hcl-project-external-childs=true",
	})
}

func TestEnvHCLProjectsAllChilds(t *testing.T) {
	runTest(t, filepath.Join("golden", "envhcl_allchilds.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples"),
		"--project-hcl-files=env.hcl",
		"--create-hcl-project-childs=true",
		"--create-hcl-project-external-childs=true",
	})
}

func TestEnvHCLProjectMarker(t *testing.T) {
	runTest(t, filepath.Join("golden", "project_marker.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "project_hcl_with_project_marker"),
		"--project-hcl-files=env.hcl",
		"--use-project-markers=true",
	})
}

func TestWithOriginalDir(t *testing.T) {
	runTest(t, filepath.Join("golden", "withOriginalDir.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "with_original_dir"),
	})
}

func TestWithExecutionOrderGroups(t *testing.T) {
	runTest(t, filepath.Join("golden", "withExecutionOrderGroups.yaml"), []string{
		"--root",
		filepath.Join("..", "test_examples", "chained_dependencies"),
		"--execution-order-groups",
	})
}
