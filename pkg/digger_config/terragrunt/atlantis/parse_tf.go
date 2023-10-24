package atlantis

import (
	"errors"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"strings"
)

var localModuleSourcePrefixes = []string{
	"./",
	"../",
	".\\",
	"..\\",
}

func parseTerraformLocalModuleSource(path string) ([]string, error) {
	module, diags := tfconfig.LoadModule(path)
	// modules, diags := parser.loadConfigDir(path)
	if diags.HasErrors() {
		return nil, errors.New(diags.Error())
	}

	var sourceMap = map[string]bool{}
	for _, mc := range module.ModuleCalls {
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

	var sources = []string{}
	for source := range sourceMap {
		sources = append(sources, source)
	}

	return sources, nil
}

func isLocalTerraformModuleSource(raw string) bool {
	for _, prefix := range localModuleSourcePrefixes {
		if strings.HasPrefix(raw, prefix) {
			return true
		}
	}

	return false
}
