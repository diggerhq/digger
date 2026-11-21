package digger_config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentRenderModeValidation(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		shouldError bool
	}{
		{
			name:        "valid basic mode",
			mode:        "basic",
			shouldError: false,
		},
		{
			name:        "valid group_by_module mode",
			mode:        "group_by_module",
			shouldError: false,
		},
		{
			name:        "invalid detailed mode",
			mode:        "detailed",
			shouldError: true,
		},
		{
			name:        "invalid random mode",
			mode:        "random",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &DiggerConfig{
				CommentRenderMode: tt.mode,
				Projects:          []Project{},
				Workflows: map[string]Workflow{
					"default": *defaultWorkflow(),
				},
			}

			err := ValidateDiggerConfig(config)
			if tt.shouldError {
				assert.Error(t, err, "Expected validation error for mode: %s", tt.mode)
				assert.Contains(t, err.Error(), "comment_render_mode", "Error should mention comment_render_mode")
			} else {
				assert.NoError(t, err, "Expected no validation error for mode: %s", tt.mode)
			}
		})
	}
}

func TestReportingConfigDefaults(t *testing.T) {
	yamlStr := `
reporting:
  ai_summary: true
`
	configYaml, err := LoadDiggerConfigYamlFromString(yamlStr)
	require.NoError(t, err)

	// Verify defaults are applied
	assert.NotNil(t, configYaml.Reporting)
	assert.True(t, configYaml.Reporting.AiSummary)
	assert.True(t, configYaml.Reporting.CommentsEnabled, "comments_enabled should default to true")
}

func TestReportingConfigExplicitFalse(t *testing.T) {
	yamlStr := `
reporting:
  ai_summary: false
  comments_enabled: false
`
	configYaml, err := LoadDiggerConfigYamlFromString(yamlStr)
	require.NoError(t, err)

	assert.NotNil(t, configYaml.Reporting)
	assert.False(t, configYaml.Reporting.AiSummary)
	assert.False(t, configYaml.Reporting.CommentsEnabled)
}

func TestDisableDiggerApplyFields(t *testing.T) {
	yamlStr := `
disable_digger_apply_comment: true
disable_digger_apply_status_check: true
projects:
  - name: test
    dir: test
`
	configYaml, err := LoadDiggerConfigYamlFromString(yamlStr)
	require.NoError(t, err)

	assert.NotNil(t, configYaml.DisableDiggerApplyComment)
	assert.True(t, *configYaml.DisableDiggerApplyComment)
	assert.NotNil(t, configYaml.DisableDiggerApplyStatusCheck)
	assert.True(t, *configYaml.DisableDiggerApplyStatusCheck)

	// Convert and verify
	config, _, err := ConvertDiggerYamlToConfig(configYaml)
	require.NoError(t, err)

	assert.True(t, config.DisableDiggerApplyComment)
	assert.True(t, config.DisableDiggerApplyStatusCheck)
}

func TestApplyRequirementsValidation(t *testing.T) {
	tests := []struct {
		name         string
		requirements []string
		shouldError  bool
	}{
		{
			name:         "valid approved",
			requirements: []string{"approved"},
			shouldError:  false,
		},
		{
			name:         "valid mergeable",
			requirements: []string{"mergeable"},
			shouldError:  false,
		},
		{
			name:         "valid undiverged",
			requirements: []string{"undiverged"},
			shouldError:  false,
		},
		{
			name:         "valid multiple",
			requirements: []string{"approved", "mergeable"},
			shouldError:  false,
		},
		{
			name:         "invalid requirement",
			requirements: []string{"invalid"},
			shouldError:  true,
		},
		{
			name:         "duplicate requirement",
			requirements: []string{"approved", "approved"},
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &DiggerConfig{
				CommentRenderMode: CommentRenderModeBasic,
				Projects: []Project{
					{
						Name:              "test",
						Dir:               "test",
						Workflow:          "default",
						ApplyRequirements: tt.requirements,
					},
				},
				Workflows: map[string]Workflow{
					"default": *defaultWorkflow(),
				},
			}

			err := ValidateDiggerConfig(config)
			if tt.shouldError {
				assert.Error(t, err, "Expected validation error for requirements: %v", tt.requirements)
			} else {
				assert.NoError(t, err, "Expected no validation error for requirements: %v", tt.requirements)
			}
		})
	}
}

func TestDependencyConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		shouldError bool
	}{
		{
			name:        "valid hard mode",
			mode:        "hard",
			shouldError: false,
		},
		{
			name:        "valid soft mode",
			mode:        "soft",
			shouldError: false,
		},
		{
			name:        "invalid mode",
			mode:        "invalid",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStr := "dependency_configuration:\n  mode: " + tt.mode
			configYaml, err := LoadDiggerConfigYamlFromString(yamlStr)
			require.NoError(t, err)

			err = ValidateDiggerConfigYaml(configYaml, "test.yml")
			if tt.shouldError {
				assert.Error(t, err, "Expected validation error for mode: %s", tt.mode)
			} else {
				assert.NoError(t, err, "Expected no validation error for mode: %s", tt.mode)
			}
		})
	}
}

func TestPulumiProjectValidation(t *testing.T) {
	t.Run("pulumi project without stack should error", func(t *testing.T) {
		config := &DiggerConfig{
			CommentRenderMode: CommentRenderModeBasic,
			Projects: []Project{
				{
					Name:     "pulumi-test",
					Dir:      "pulumi",
					Workflow: "default",
					Pulumi:   true,
					// Missing PulumiStack
				},
			},
			Workflows: map[string]Workflow{
				"default": *defaultWorkflow(),
			},
		}

		err := ValidateDiggerConfig(config)
		assert.Error(t, err, "Pulumi project without stack should error")
		assert.Contains(t, err.Error(), "pulumi stack", "Error should mention pulumi stack")
	})

	t.Run("pulumi project with stack should be valid", func(t *testing.T) {
		config := &DiggerConfig{
			CommentRenderMode: CommentRenderModeBasic,
			Projects: []Project{
				{
					Name:        "pulumi-test",
					Dir:         "pulumi",
					Workflow:    "default",
					Pulumi:      true,
					PulumiStack: "dev",
				},
			},
			Workflows: map[string]Workflow{
				"default": *defaultWorkflow(),
			},
		}

		err := ValidateDiggerConfig(config)
		assert.NoError(t, err, "Pulumi project with stack should be valid")
	})
}

func TestMultipleIacTypesValidation(t *testing.T) {
	config := &DiggerConfig{
		CommentRenderMode: CommentRenderModeBasic,
		Projects: []Project{
			{
				Name:       "multi-iac",
				Dir:        "test",
				Workflow:   "default",
				Terragrunt: true,
				OpenTofu:   true,
				Pulumi:     true,
			},
		},
		Workflows: map[string]Workflow{
			"default": *defaultWorkflow(),
		},
	}

	err := ValidateDiggerConfig(config)
	assert.Error(t, err, "Project with multiple IAC types should error")
	assert.Contains(t, err.Error(), "more than one IAC", "Error should mention multiple IAC types")
}
