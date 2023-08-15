package atlantis

import (
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"path/filepath"
)

const bareIncludeKey = ""

type parsedHcl struct {
	Terraform *config.TerraformConfig `hcl:"terraform,block"`
	Includes  []config.IncludeConfig  `hcl:"include,block"`
}

// terragruntIncludeMultiple is a struct that can be used to only decode the include block with labels.
type terragruntIncludeMultiple struct {
	Include []config.IncludeConfig `hcl:"include,block"`
	Remain  hcl.Body               `hcl:",remain"`
}

// updateBareIncludeBlock searches the parsed terragrunt contents for a bare include block (include without a label),
// and convert it to one with empty string as the label. This is necessary because the hcl parser is strictly enforces
// label counts when parsing out labels with a go struct.
//
// Returns the updated contents, a boolean indicated whether anything changed, and an error (if any).
func updateBareIncludeBlock(file *hcl.File, filename string) ([]byte, bool, error) {
	hclFile, err := hclwrite.ParseConfig(file.Bytes, filename, hcl.InitialPos)
	if err != nil {
		return nil, false, errors.WithStackTrace(err)
	}

	codeWasUpdated := false
	for _, block := range hclFile.Body().Blocks() {
		if block.Type() == "include" && len(block.Labels()) == 0 {
			if codeWasUpdated {
				return nil, false, errors.WithStackTrace(config.MultipleBareIncludeBlocksErr{})
			}
			block.SetLabels([]string{bareIncludeKey})
			codeWasUpdated = true
		}
	}
	return hclFile.Bytes(), codeWasUpdated, nil
}

// decodeHcl uses the HCL2 parser to decode the parsed HCL into the struct specified by out.
//
// Note that we take a two pass approach to support parsing include blocks without a label. Ideally we can parse include
// blocks with and without labels in a single pass, but the HCL parser is fairly restrictive when it comes to parsing
// blocks with labels, requiring the exact number of expected labels in the parsing step.  To handle this restriction,
// we first see if there are any include blocks without any labels, and if there is, we modify it in the file object to
// inject the label as "".
func decodeHcl(
	file *hcl.File,
	filename string,
	out interface{},
	terragruntOptions *options.TerragruntOptions,
	extensions config.EvalContextExtensions,
) (err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.WithStackTrace(config.PanicWhileParsingConfig{RecoveredValue: recovered, ConfigFile: filename})
		}
	}()

	// Check if we need to update the file to label any bare include blocks.
	// Excluding json because of https://github.com/transcend-io/terragrunt-atlantis-config/issues/244.
	if filepath.Ext(filename) != ".json" {
		updatedBytes, isUpdated, err := updateBareIncludeBlock(file, filename)
		if err != nil {
			return err
		}
		if isUpdated {
			// Code was updated, so we need to reparse the new updated contents. This is necessarily because the blocks
			// returned by hclparse does not support editing, and so we have to go through hclwrite, which leads to a
			// different AST representation.
			file, err = parseHcl(hclparse.NewParser(), string(updatedBytes), filename)
			if err != nil {
				return err
			}
		}
	}

	evalContext, err := config.CreateTerragruntEvalContext(filename, terragruntOptions, extensions)
	if err != nil {
		return err
	}

	decodeDiagnostics := gohcl.DecodeBody(file.Body, evalContext, out)
	if decodeDiagnostics != nil && decodeDiagnostics.HasErrors() {
		return decodeDiagnostics
	}

	return nil
}

// This decodes only the `include` blocks of a terragrunt config, so its value can be used while decoding the rest of
// the config.
// For consistency, `include` in the call to `decodeHcl` is always assumed to be nil. Either it really is nil (parsing
// the child config), or it shouldn't be used anyway (the parent config shouldn't have an include block).
func decodeAsTerragruntInclude(
	file *hcl.File,
	filename string,
	terragruntOptions *options.TerragruntOptions,
	extensions config.EvalContextExtensions,
) ([]config.IncludeConfig, error) {
	tgInc := terragruntIncludeMultiple{}
	if err := decodeHcl(file, filename, &tgInc, terragruntOptions, extensions); err != nil {
		return nil, err
	}
	return tgInc.Include, nil
}

// Not all modules need an include statement, as they could define everything in one file without a parent
// The key signifiers of a parent are:
//   - no include statement
//   - no terraform source defined
// If both of those are true, it is likely a parent module
func parseModule(path string, terragruntOptions *options.TerragruntOptions) (isParent bool, includes []config.IncludeConfig, err error) {
	configString, err := util.ReadFileAsString(path)
	if err != nil {
		return false, nil, err
	}

	parser := hclparse.NewParser()
	file, err := parseHcl(parser, configString, path)
	if err != nil {
		return false, nil, err
	}

	// Decode just the `include` and `import` blocks, and verify that it's allowed here
	extensions := config.EvalContextExtensions{}
	terragruntIncludeList, err := decodeAsTerragruntInclude(file, path, terragruntOptions, extensions)
	if err != nil {
		return false, nil, err
	}

	// If the file has any `include` blocks it is not a parent
	if len(terragruntIncludeList) > 0 {
		return false, terragruntIncludeList, nil
	}

	// We don't need to check the errors/diagnostics coming from `decodeHcl`, as when errors come up,
	// it will leave the partially parsed result in the output object.
	var parsed parsedHcl
	decodeHcl(file, path, &parsed, terragruntOptions, extensions)

	// If the file does not define a terraform source block, it is likely a parent (though not guaranteed)
	if parsed.Terraform == nil || parsed.Terraform.Source == nil {
		return true, nil, nil
	}

	return false, nil, nil
}
