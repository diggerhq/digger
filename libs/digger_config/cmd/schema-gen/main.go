package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	digger_config "github.com/diggerhq/digger/libs/digger_config"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	var (
		output   = flag.String("output", "digger-schema.json", "Output file path")
		validate = flag.Bool("validate", false, "Validate existing schema instead of generating")
	)
	flag.Parse()

	if *validate {
		if err := validateSchema(*output); err != nil {
			fmt.Fprintf(os.Stderr, "Schema validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Schema validation passed")
		return
	}

	if err := generateSchema(*output); err != nil {
		fmt.Fprintf(os.Stderr, "Schema generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Schema generated: %s\n", *output)
}

func generateSchema(outputPath string) error {
	// Generate schema from Go struct
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
		FieldNameTag:              "yaml",
	}

	schema := reflector.Reflect(&digger_config.DiggerConfigYaml{})

	// Customize schema
	schema.ID = "https://docs.opentaco.dev/schemas/digger.yml.json"
	schema.Title = "Digger Configuration"
	schema.Description = "Configuration schema for digger.yml - the source of truth for Digger configuration validation"

	// Add custom validations and descriptions
	enhanceSchema(schema)

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(schema, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	return nil
}

func enhanceSchema(schema *jsonschema.Schema) {
	// Add enum constraints and descriptions based on code constants
	if props := schema.Properties; props != nil {
		// comment_render_mode
		if prop, ok := props.Get("comment_render_mode"); ok && prop != nil {
			prop.Enum = []interface{}{
				digger_config.CommentRenderModeBasic,
				digger_config.CommentRenderModeGroupByModule,
			}
			prop.Default = digger_config.CommentRenderModeBasic
			prop.Description = "How to render plan output in PR comments"
		}

		// dependency_configuration
		if prop, ok := props.Get("dependency_configuration"); ok && prop != nil && prop.Properties != nil {
			if modeProp, modeOk := prop.Properties.Get("mode"); modeOk && modeProp != nil {
				modeProp.Enum = []interface{}{"hard", "soft"}
				modeProp.Default = "hard"
				modeProp.Description = "Dependency resolution mode: 'hard' executes dependencies even if unchanged, 'soft' skips unchanged dependencies"
			}
		}

		// auto_merge_strategy
		if prop, ok := props.Get("auto_merge_strategy"); ok && prop != nil {
			prop.Enum = []interface{}{"squash", "merge", "rebase"}
			prop.Default = "squash"
			prop.Description = "Strategy to use when auto-merging PRs"
		}

		// apply_requirements (in projects)
		if projectsProp, ok := props.Get("projects"); ok && projectsProp != nil && projectsProp.Items != nil {
			if projectProps := projectsProp.Items.Properties; projectProps != nil {
				if applyReqProp, applyOk := projectProps.Get("apply_requirements"); applyOk && applyReqProp != nil && applyReqProp.Items != nil {
					applyReqProp.Items.Enum = []interface{}{
						digger_config.ApplyRequirementsApproved,
						digger_config.ApplyRequirementsMergeable,
						digger_config.ApplyRequirementsUndiverged,
					}
					applyReqProp.Description = "Requirements that must be met before apply"
				}
			}
		}

		// Add descriptions for boolean fields
		addBoolDescription(props, "apply_after_merge", "Automatically apply changes after PR is merged", false)
		addBoolDescription(props, "allow_draft_prs", "Allow Digger to run on draft pull requests", false)
		addBoolDescription(props, "delete_prior_comments", "Delete previous Digger comments when posting new ones", false)
		addBoolDescription(props, "disable_digger_apply_comment", "Disable the 'digger apply' comment command", false)
		addBoolDescription(props, "disable_digger_apply_status_check", "Disable the apply status check on PRs", false)
		addBoolDescription(props, "pr_locks", "Enable PR-level locking to prevent concurrent runs", true)
		addBoolDescription(props, "auto_merge", "Automatically merge PRs after successful apply", false)
		addBoolDescription(props, "telemetry", "Enable telemetry reporting", true)
		addBoolDescription(props, "traverse_to_nested_projects", "When generating projects, traverse into nested directories", false)
		addBoolDescription(props, "mention_drifted_projects_in_pr", "Mention projects with drift in PR comments", false)
		addBoolDescription(props, "report_terraform_outputs", "Include Terraform outputs in PR comments", true)
		addBoolDescription(props, "respect_layers", "Respect layer ordering when executing projects", false)
	}
}

func addBoolDescription(props *orderedmap.OrderedMap[string, *jsonschema.Schema], key, description string, defaultVal bool) {
	if prop, ok := props.Get(key); ok && prop != nil {
		prop.Description = description
		prop.Default = defaultVal
	}
}

func validateSchema(schemaPath string) error {
	// Read existing schema
	existingData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read existing schema: %w", err)
	}

	var existingSchema jsonschema.Schema
	if err := json.Unmarshal(existingData, &existingSchema); err != nil {
		return fmt.Errorf("failed to parse existing schema: %w", err)
	}

	// Generate new schema
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
		FieldNameTag:              "yaml",
	}
	newSchema := reflector.Reflect(&digger_config.DiggerConfigYaml{})
	enhanceSchema(newSchema)

	// Compare key properties
	if err := compareSchemas(&existingSchema, newSchema); err != nil {
		return fmt.Errorf("schema mismatch: %w", err)
	}

	return nil
}

func compareSchemas(existing, generated *jsonschema.Schema) error {
	// Check that all properties in generated schema exist in existing
	if generated.Properties != nil {
		for pair := generated.Properties.Oldest(); pair != nil; pair = pair.Next() {
			key := pair.Key
			if existing.Properties == nil {
				return fmt.Errorf("missing property in existing schema: %s\n\nThe schema is out of date. Run: make schema-generate", key)
			}
			if _, ok := existing.Properties.Get(key); !ok {
				return fmt.Errorf("missing property in existing schema: %s\n\nThe schema is out of date. Run: make schema-generate", key)
			}
		}
	}

	// Validate critical enum values
	if err := validateEnumProperty(existing, "comment_render_mode",
		digger_config.CommentRenderModeBasic,
		digger_config.CommentRenderModeGroupByModule); err != nil {
		return fmt.Errorf("%w\n\nThe schema is out of date. Run: make schema-generate", err)
	}

	return nil
}

func validateEnumProperty(schema *jsonschema.Schema, propName string, expectedValues ...string) error {
	if schema.Properties == nil {
		return fmt.Errorf("schema has no properties")
	}

	prop, ok := schema.Properties.Get(propName)
	if !ok || prop == nil {
		return fmt.Errorf("property '%s' not found in schema", propName)
	}

	if prop.Enum == nil {
		return fmt.Errorf("property '%s' has no enum values defined", propName)
	}

	// Check all expected values are present
	enumMap := make(map[string]bool)
	for _, v := range prop.Enum {
		if str, ok := v.(string); ok {
			enumMap[str] = true
		}
	}

	for _, expected := range expectedValues {
		if !enumMap[expected] {
			return fmt.Errorf("property '%s' missing required enum value: '%s' (has: %v)", propName, expected, prop.Enum)
		}
	}

	return nil
}
