package tac

import (
	"errors"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"os"
	"path/filepath"
	"strings"
)

var localModuleSourcePrefixes = []string{
	"./",
	"../",
	".\\",
	"..\\",
}

// moduleCall represents a module block with its source
type moduleCall struct {
	Source string
}

// parseTerraformLocalModuleSource parses Terraform files using hclparse to extract local module sources.
// This replaces the use of terraform-config-inspect which doesn't handle for_each in provider blocks.
func parseTerraformLocalModuleSource(path string) ([]string, error) {
	var sourceMap = map[string]bool{}

	// Read all .tf and .tf.json files in the directory
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	parser := hclparse.NewParser()

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		ext := filepath.Ext(filename)
		
		// Only process Terraform files
		if ext != ".tf" && ext != ".json" {
			continue
		}

		fullPath := filepath.Join(path, filename)
		
		// Read file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue // Skip files we can't read
		}

		// Parse the HCL file
		var hclFile *hcl.File
		var diags hcl.Diagnostics

		if ext == ".json" {
			hclFile, diags = parser.ParseJSON(content, fullPath)
		} else {
			hclFile, diags = parser.ParseHCL(content, fullPath)
		}

		// If there are errors, return them
		if diags.HasErrors() {
			return nil, errors.New(diags.Error())
		}

		// Extract module calls from the parsed file
		moduleCalls := extractModuleCalls(hclFile.Body)
		
		for _, mc := range moduleCalls {
			if isLocalTerraformModuleSource(mc.Source) {
				modulePath := util.JoinPath(path, mc.Source)
				modulePathGlob := util.JoinPath(modulePath, "*.tf*")

				if _, exists := sourceMap[modulePathGlob]; exists {
					continue
				}
				sourceMap[modulePathGlob] = true

				// find local module source recursively
				subSources, err := parseTerraformLocalModuleSource(modulePath)
				if err != nil {
					return nil, err
				}

				for _, subSource := range subSources {
					sourceMap[subSource] = true
				}
			}
		}
	}

	var sources = []string{}
	for source := range sourceMap {
		sources = append(sources, source)
	}

	return sources, nil
}

// extractModuleCalls extracts module blocks from an HCL body
func extractModuleCalls(body hcl.Body) []moduleCall {
	var modules []moduleCall

	// We need to work with the native syntax to extract blocks
	content, ok := body.(*hclsyntax.Body)
	if !ok {
		return modules
	}

	for _, block := range content.Blocks {
		if block.Type == "module" {
			// Look for the source attribute in the block
			if attr, exists := block.Body.Attributes["source"]; exists {
				// Try to extract the literal value
				val, diags := attr.Expr.Value(nil)
				if diags.HasErrors() || val.IsNull() {
					continue
				}

				if val.Type() == cty.String {
					modules = append(modules, moduleCall{
						Source: val.AsString(),
					})
				}
			}
		}
	}

	return modules
}

func isLocalTerraformModuleSource(raw string) bool {
	for _, prefix := range localModuleSourcePrefixes {
		if strings.HasPrefix(raw, prefix) {
			return true
		}
	}

	return false
}
