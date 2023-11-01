package github

import (
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
	"testing"
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

	impactedProjectsWithDependants, err := FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
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
