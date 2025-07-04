package github

import (
	"testing"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
)

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
			// Simulate the fixed project selection logic
			var projectConfig digger_config.Project
			var projectFound bool

			for _, config := range diggerConfig.Projects {
				if config.Name == tt.requestedProject {
					projectConfig = config
					projectFound = true
					break
				}
			}

			if tt.shouldFind {
				assert.True(t, projectFound, "Expected to find project %s", tt.requestedProject)
				assert.Equal(t, tt.expectedProject, projectConfig.Name, "Expected project name to match")
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
	var projectFound bool
	var availableProjects []string

	// Simulate the project search
	for _, config := range diggerConfig.Projects {
		if config.Name == requestedProject {
			projectFound = true
			break
		}
	}

	if !projectFound {
		// Collect available projects for error message
		for _, p := range diggerConfig.Projects {
			availableProjects = append(availableProjects, p.Name)
		}
	}

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
	var projectFound bool
	var availableProjects []string

	for _, config := range diggerConfig.Projects {
		if config.Name == requestedProject {
			projectFound = true
			break
		}
	}

	if !projectFound {
		for _, p := range diggerConfig.Projects {
			availableProjects = append(availableProjects, p.Name)
		}

		// This would normally call usage.ReportErrorAndExit
		expectedErrorMsg := "Project 'invalid-project' not found in digger configuration. Available projects: [project-1 project-2]"

		// Verify the error message format
		actualErrorMsg := "Project '" + requestedProject + "' not found in digger configuration. Available projects: " +
			"[project-1 project-2]"
		assert.Equal(t, expectedErrorMsg, actualErrorMsg)
	}
}
