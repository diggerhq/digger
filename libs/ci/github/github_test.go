package github

import (
	"github.com/diggerhq/digger/libs/ci/generic"
	"strings"
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

func TestSplitComment_ShortComment(t *testing.T) {
	parts := splitComment("short comment", 100)
	assert.Equal(t, 1, len(parts))
	assert.Equal(t, "short comment", parts[0])
}

func TestSplitComment_LongComment(t *testing.T) {
	// Create a comment that exceeds maxLen, with newlines
	line := "This is a line of text for testing.\n"
	comment := ""
	for i := 0; i < 100; i++ {
		comment += line
	}
	maxLen := 500
	parts := splitComment(comment, maxLen)

	assert.Greater(t, len(parts), 1)
	for _, part := range parts {
		assert.LessOrEqual(t, len(part), maxLen)
	}
	// Verify no content is lost
	joined := ""
	for _, part := range parts {
		joined += part
	}
	assert.Equal(t, comment, joined)
}

func TestSplitComment_ExactLimit(t *testing.T) {
	comment := strings.Repeat("a", 100)
	parts := splitComment(comment, 100)
	assert.Equal(t, 1, len(parts))
	assert.Equal(t, comment, parts[0])
}
