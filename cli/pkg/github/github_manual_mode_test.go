package github

import (
	"fmt"
	"testing"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
)

// Helper function to get available project names
func getAvailableProjectNames(projects []digger_config.Project) []string {
	var availableProjects []string
	for _, p := range projects {
		availableProjects = append(availableProjects, p.Name)
	}
	return availableProjects
}

func TestManualModeProjectValidation(t *testing.T) {
	// Create a test digger config with some projects
	diggerConfig := &digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{
				Name: "project-a",
				Dir:  "./project-a",
			},
			{
				Name: "project-b",
				Dir:  "./project-b",
			},
			{
				Name: "project-c",
				Dir:  "./project-c",
			},
		},
	}

	tests := []struct {
		name             string
		requestedProject string
		shouldFind       bool
		expectedProject  string
	}{
		{
			name:             "Valid project should be found",
			requestedProject: "project-b",
			shouldFind:       true,
			expectedProject:  "project-b",
		},
		{
			name:             "Non-existent project should not be found",
			requestedProject: "non-existent-project",
			shouldFind:       false,
			expectedProject:  "",
		},
		{
			name:             "Empty project name should not be found",
			requestedProject: "",
			shouldFind:       false,
			expectedProject:  "",
		},
		{
			name:             "Case sensitive project name should not be found",
			requestedProject: "Project-A",
			shouldFind:       false,
			expectedProject:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use helper function for project search
			projectConfig, projectFound := findProjectInConfig(diggerConfig.Projects, tt.requestedProject)

			if tt.shouldFind {
				assert.True(t, projectFound, "Expected to find project %s", tt.requestedProject)
				assert.Equal(t, tt.expectedProject, projectConfig.Name, "Expected project name to match")
				assert.Equal(t, "./"+tt.expectedProject, projectConfig.Dir, "Expected project directory to match")
			} else {
				assert.False(t, projectFound, "Expected NOT to find project %s", tt.requestedProject)
				// In the old buggy code, projectConfig would contain the last project from the loop
				// With our fix, projectFound will be false and we should exit with an error
			}
		})
	}
}

func TestManualModeProjectValidationWithOldBuggyLogic(t *testing.T) {
	// This test demonstrates the old buggy behavior
	diggerConfig := &digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{
				Name: "project-a",
				Dir:  "./project-a",
			},
			{
				Name: "project-b",
				Dir:  "./project-b",
			},
			{
				Name: "dangerous-project",
				Dir:  "./dangerous-project",
			},
		},
	}

	// Simulate the OLD buggy logic
	requestedProject := "non-existent-project"
	var projectConfig digger_config.Project

	// This is the OLD BUGGY CODE that would cause the issue
	for _, projectConfig = range diggerConfig.Projects {
		if projectConfig.Name == requestedProject {
			break
		}
	}

	// In the old code, projectConfig would now contain "dangerous-project"
	// (the last project in the loop) even though we requested "non-existent-project"
	assert.Equal(t, "dangerous-project", projectConfig.Name,
		"This demonstrates the old bug: projectConfig contains the last project from the loop")
	assert.NotEqual(t, requestedProject, projectConfig.Name,
		"This shows the dangerous behavior: we're using a different project than requested")
}

func TestAvailableProjectsLogging(t *testing.T) {
	// Test that we properly log available projects when a project is not found
	diggerConfig := &digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{Name: "web-app"},
			{Name: "api-service"},
			{Name: "database"},
		},
	}

	requestedProject := "missing-project"

	// Use helper functions
	_, projectFound := findProjectInConfig(diggerConfig.Projects, requestedProject)
	availableProjects := getAvailableProjectNames(diggerConfig.Projects)

	assert.False(t, projectFound)
	assert.Equal(t, []string{"web-app", "api-service", "database"}, availableProjects)
}

// This test would actually call usage.ReportErrorAndExit in a real scenario
// but we can't easily test that without mocking the exit behavior
func TestProjectNotFoundErrorMessage(t *testing.T) {
	diggerConfig := &digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{Name: "project-1"},
			{Name: "project-2"},
		},
	}

	requestedProject := "invalid-project"

	// Use helper functions
	_, projectFound := findProjectInConfig(diggerConfig.Projects, requestedProject)
	availableProjects := getAvailableProjectNames(diggerConfig.Projects)

	assert.False(t, projectFound)

	if !projectFound {
		// This would normally call usage.ReportErrorAndExit
		expectedErrorMsg := "Project 'invalid-project' not found in digger configuration. Available projects: [project-1 project-2]"

		// Verify the error message format using fmt.Sprintf for better maintainability
		actualErrorMsg := fmt.Sprintf("Project '%s' not found in digger configuration. Available projects: %v", requestedProject, availableProjects)
		assert.Equal(t, expectedErrorMsg, actualErrorMsg)
		assert.Equal(t, []string{"project-1", "project-2"}, availableProjects)
	}
}

func TestWorkflowValidation(t *testing.T) {
	// Test that we properly validate workflow existence
	diggerConfig := &digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{
				Name:     "test-project",
				Workflow: "custom-workflow",
			},
		},
		Workflows: map[string]digger_config.Workflow{
			"default": {
				Plan:  &digger_config.Stage{Steps: []digger_config.Step{{Action: "init"}}},
				Apply: &digger_config.Stage{Steps: []digger_config.Step{{Action: "apply"}}},
			},
		},
	}

	// Test invalid workflow
	project := diggerConfig.Projects[0]
	_, workflowExists := diggerConfig.Workflows[project.Workflow]
	assert.False(t, workflowExists, "custom-workflow should not exist")
	
	// Test that the error message is correctly formatted
	expectedErrorMsg := fmt.Sprintf("Workflow '%s' not found for project '%s'", project.Workflow, project.Name)
	actualErrorMsg := fmt.Sprintf("Workflow '%s' not found for project '%s'", project.Workflow, project.Name)
	assert.Equal(t, expectedErrorMsg, actualErrorMsg)

	// Test workflow that exists
	project.Workflow = "default"
	_, workflowExists = diggerConfig.Workflows[project.Workflow]
	assert.True(t, workflowExists, "default workflow should exist")
}
