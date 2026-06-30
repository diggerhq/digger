package github

import (
	"os"
	"testing"

	"github.com/diggerhq/digger/libs/ci/generic"
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

	impactedProjectsWithDependants, err := generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
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

func TestFindAllChangedFilesOfPR(t *testing.T) {
	githubPrService, _ := GithubServiceProviderBasic{}.NewService("", "digger", "diggerhq")
	files, _ := githubPrService.GetChangedFiles(98)
	// 45 changed files including 1 renamed file so the previous filename is included
	assert.Equal(t, 46, len(files))
}

func TestConfigureEnterpriseClient_WithDiggerHostname(t *testing.T) {
	// Set up
	os.Setenv("DIGGER_GITHUB_HOSTNAME", "github.example.com")
	defer os.Unsetenv("DIGGER_GITHUB_HOSTNAME")

	svc, err := GithubServiceProviderBasic{}.NewService("test-token", "repo", "owner")

	assert.NoError(t, err)
	assert.NotNil(t, svc.Client)
	assert.Equal(t, "https://github.example.com/api/v3/", svc.Client.BaseURL.String())
	assert.Equal(t, "https://github.example.com/api/uploads/", svc.Client.UploadURL.String())
}

func TestConfigureEnterpriseClient_WithGitHubApiUrl(t *testing.T) {
	// Set up - this simulates GitHub Actions environment on Enterprise
	os.Setenv("GITHUB_API_URL", "https://github.example.com/api/v3")
	defer os.Unsetenv("GITHUB_API_URL")

	svc, err := GithubServiceProviderBasic{}.NewService("test-token", "repo", "owner")

	assert.NoError(t, err)
	assert.NotNil(t, svc.Client)
	assert.Equal(t, "https://github.example.com/api/v3/", svc.Client.BaseURL.String())
	assert.Equal(t, "https://github.example.com/api/uploads/", svc.Client.UploadURL.String())
}

func TestConfigureEnterpriseClient_PublicGitHub(t *testing.T) {
	// Ensure no enterprise env vars are set
	os.Unsetenv("DIGGER_GITHUB_HOSTNAME")
	os.Unsetenv("GITHUB_API_URL")

	svc, err := GithubServiceProviderBasic{}.NewService("test-token", "repo", "owner")

	assert.NoError(t, err)
	assert.NotNil(t, svc.Client)
	assert.Equal(t, "https://api.github.com/", svc.Client.BaseURL.String())
}

func TestConfigureEnterpriseClient_PublicGitHubApiUrl(t *testing.T) {
	// GITHUB_API_URL set to public GitHub should not trigger enterprise config
	os.Setenv("GITHUB_API_URL", "https://api.github.com")
	defer os.Unsetenv("GITHUB_API_URL")

	svc, err := GithubServiceProviderBasic{}.NewService("test-token", "repo", "owner")

	assert.NoError(t, err)
	assert.NotNil(t, svc.Client)
	assert.Equal(t, "https://api.github.com/", svc.Client.BaseURL.String())
}

func TestConfigureEnterpriseClient_DiggerHostnameTakesPrecedence(t *testing.T) {
	// Both set - DIGGER_GITHUB_HOSTNAME should take precedence
	os.Setenv("DIGGER_GITHUB_HOSTNAME", "digger-enterprise.example.com")
	os.Setenv("GITHUB_API_URL", "https://actions-enterprise.example.com/api/v3")
	defer os.Unsetenv("DIGGER_GITHUB_HOSTNAME")
	defer os.Unsetenv("GITHUB_API_URL")

	svc, err := GithubServiceProviderBasic{}.NewService("test-token", "repo", "owner")

	assert.NoError(t, err)
	assert.Equal(t, "https://digger-enterprise.example.com/api/v3/", svc.Client.BaseURL.String())
}
