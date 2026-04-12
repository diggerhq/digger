package github

import (
	"github.com/diggerhq/digger/libs/ci/generic"
	"os"
	"path"
	"testing"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
)

func TestFindAllProjectsDependantOnImpactedProjects(t *testing.T) {

	projects := []digger_config.Project{
		{
			Name: "a",
		},
		{
			Name:               "b",
			DependencyProjects: []string{"k"},
		},
		{
			Name:               "c",
			DependencyProjects: []string{"b", "a", "i"},
		},
		{
			Name:               "d",
			DependencyProjects: []string{"c"},
		},
		{
			Name:               "e",
			DependencyProjects: []string{"i", "c"},
		},
		{
			Name:               "f",
			DependencyProjects: []string{"e"},
		},
		{
			Name:               "g",
			DependencyProjects: []string{"e"},
		},
		{
			Name: "h",
		},
		{
			Name: "i",
		},
		{
			Name: "j",
		},
		{
			Name: "k",
		},
		{
			Name:               "m",
			DependencyProjects: []string{"h"},
		},
	}

	dependencyGraph, err := digger_config.CreateProjectDependencyGraph(projects)

	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	impactedProjects := []digger_config.Project{
		{
			Name: "a",
		},
		{
			Name: "d",
		},
		{
			Name: "f",
		},
		{
			Name: "g",
		},
		{
			Name: "h",
		},
		{
			Name: "i",
		},
		{
			Name: "j",
		},
		{
			Name: "m",
		},
	}

	impactedProjectsWithDependants, err := generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph, nil)
	if err != nil {
		return
	}

	assert.Equal(t, 10, len(impactedProjectsWithDependants))

	projectNames := make([]string, 10)
	for _, project := range impactedProjectsWithDependants {
		projectNames = append(projectNames, project.Name)
	}

	assert.Contains(t, projectNames, "a")
	assert.Contains(t, projectNames, "c")
	assert.Contains(t, projectNames, "d")
	assert.Contains(t, projectNames, "e")
	assert.Contains(t, projectNames, "f")
	assert.Contains(t, projectNames, "g")
	assert.Contains(t, projectNames, "h")
	assert.Contains(t, projectNames, "i")
	assert.Contains(t, projectNames, "j")
	assert.Contains(t, projectNames, "m")
	assert.NotContains(t, projectNames, "k")
	assert.NotContains(t, projectNames, "b")
}

func TestFindAllProjectsDependantOnImpactedProjectsRespectsExcludedDependencyFileTriggers(t *testing.T) {
	tempDir := t.TempDir()

	diggerCfg := `
dependency_configuration:
  mode: hard
projects:
- name: shared
  dir: modules/shared
- name: consumer
  dir: env/dev
  dependency_file_triggers: true
  exclude_patterns:
    - modules/shared/outputs.tf
`

	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "modules", "shared"), 0o755))
	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "env", "dev"), 0o755))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "digger.yml"), []byte(diggerCfg), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "modules", "shared", "main.tf"), []byte(`resource "null_resource" "shared" {}`), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "modules", "shared", "outputs.tf"), []byte(`output "shared" { value = "x" }`), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "env", "dev", "main.tf"), []byte(`module "shared" { source = "../../modules/shared" }`), 0o644))

	config, _, dependencyGraph, _, err := digger_config.LoadDiggerConfig(tempDir, true, nil, nil)
	assert.NoError(t, err)

	impactedProjects, _ := config.GetModifiedProjects([]string{"modules/shared/outputs.tf"})
	assert.Len(t, impactedProjects, 1)
	assert.Equal(t, "shared", impactedProjects[0].Name)

	impactedProjectsWithDependants, err := generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph, []string{"modules/shared/outputs.tf"})
	assert.NoError(t, err)
	assert.Len(t, impactedProjectsWithDependants, 1)
	assert.Equal(t, "shared", impactedProjectsWithDependants[0].Name)
}

func TestFindAllProjectsDependantOnImpactedProjectsOnlyPropagatesMatchingImportedFiles(t *testing.T) {
	tempDir := t.TempDir()

	diggerCfg := `
dependency_configuration:
  mode: hard
projects:
- name: shared
  dir: modules/shared
- name: consumer
  dir: env/dev
  dependency_file_triggers: true
`

	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "modules", "shared"), 0o755))
	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "env", "dev"), 0o755))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "digger.yml"), []byte(diggerCfg), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "modules", "shared", "main.tf"), []byte(`resource "null_resource" "shared" {}`), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "modules", "shared", "README.md"), []byte(`shared docs`), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "env", "dev", "main.tf"), []byte(`module "shared" { source = "../../modules/shared" }`), 0o644))

	config, _, dependencyGraph, _, err := digger_config.LoadDiggerConfig(tempDir, true, nil, nil)
	assert.NoError(t, err)

	impactedProjects, _ := config.GetModifiedProjects([]string{"modules/shared/README.md"})
	assert.Len(t, impactedProjects, 1)
	assert.Equal(t, "shared", impactedProjects[0].Name)

	impactedProjectsWithDependants, err := generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph, []string{"modules/shared/README.md"})
	assert.NoError(t, err)
	assert.Len(t, impactedProjectsWithDependants, 1)
	assert.Equal(t, "shared", impactedProjectsWithDependants[0].Name)

	impactedProjects, _ = config.GetModifiedProjects([]string{"modules/shared/main.tf"})
	assert.Len(t, impactedProjects, 2)

	impactedProjectsWithDependants, err = generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph, []string{"modules/shared/main.tf"})
	assert.NoError(t, err)
	assert.Len(t, impactedProjectsWithDependants, 2)
	projectNames := []string{impactedProjectsWithDependants[0].Name, impactedProjectsWithDependants[1].Name}
	assert.Contains(t, projectNames, "shared")
	assert.Contains(t, projectNames, "consumer")
}

func TestFindAllChangedFilesOfPR(t *testing.T) {
	githubPrService, _ := GithubServiceProviderBasic{}.NewService("", "digger", "diggerhq")
	files, _ := githubPrService.GetChangedFiles(98)
	// 45 changed files including 1 renamed file so the previous filename is included
	assert.Equal(t, 46, len(files))
}
