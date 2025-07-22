package atlantis

import (
	"fmt"
	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
	"log/slog"
	"strings"
)

const (
	// A consistent error message for multiple catalog block in terragrunt config (which is currently not supported)
	multipleBlockDetailFmt = "Terragrunt currently does not support multiple %[1]s blocks in a single config. Consolidate to a single %[1]s block."
)

const (
	// A consistent detail message for all "not a valid identifier" diagnostics. This is exactly the same as that returned
	// by terraform.
	badIdentifierDetail = "A name must start with a letter and may contain only letters, digits, underscores, and dashes."
)

// getLocalName takes a variable reference encoded as a HCL tree traversal that is rooted at the name `local` and
// returns the underlying variable lookup on the local map. If it is not a local name lookup, this will return empty
// string.
func getLocalName(traversal hcl.Traversal) string {
	if traversal.IsRelative() {
		return ""
	}

	if traversal.RootName() != "local" {
		return ""
	}

	split := traversal.SimpleSplit()
	for _, relRaw := range split.Rel {
		switch rel := relRaw.(type) {
		case hcl.TraverseAttr:
			return rel.Name
		default:
			// This means that it is either an operation directly on the locals block, or is an unsupported action (e.g
			// a splat or lookup). Either way, there is no local name.
			continue
		}
	}
	return ""
}

// canEvaluateLocals determines if the local expression can be evaluated. An expression can be evaluated if one of the
// following is true:
// - It has no references to other locals.
// - It has references to other locals that have already been evaluated.
// Note that the second return value is a human friendly reason for why the expression can not be evaluated, and is
// useful for error reporting.
func canEvaluateLocals(expression hcl.Expression,
	evaluatedLocals map[string]cty.Value,
) (bool, string) {
	vars := expression.Variables()
	if len(vars) == 0 {
		// If there are no local variable references, we can evaluate this expression.
		return true, ""
	}

	for _, var_ := range vars {
		// This should never happen, but if it does, we can't evaluate this expression.
		if var_.IsRelative() {
			reason := "You've reached an impossible condition and is almost certainly a bug in terragrunt. Please open an issue at github.com/gruntwork-io/terragrunt with this message and the contents of your terragrunt.hcl file that caused this."
			return false, reason
		}

		rootName := var_.RootName()

		// If the variable is `include`, then we can evaluate it now
		if rootName == "include" {
			continue
		}

		// We can't evaluate any variable other than `local`
		if rootName != "local" {
			reason := fmt.Sprintf(
				"Can't evaluate expression at %s: you can only reference other local variables here, but it looks like you're referencing something else (%s is not defined)",
				expression.Range(),
				rootName,
			)
			return false, reason
		}

		// If we can't get any local name, we can't evaluate it.
		localName := getLocalName(var_)
		if localName == "" {
			reason := fmt.Sprintf(
				"Can't evaluate expression at %s because local var name can not be determined.",
				expression.Range(),
			)
			return false, reason
		}

		// If the referenced local isn't evaluated, we can't evaluate this expression.
		_, hasEvaluated := evaluatedLocals[localName]
		if !hasEvaluated {
			reason := fmt.Sprintf(
				"Can't evaluate expression at %s because local reference '%s' is not evaluated. Either it is not ready yet in the current pass, or there was an error evaluating it in an earlier stage.",
				expression.Range(),
				localName,
			)
			return false, reason
		}
	}

	// If we made it this far, this means all the variables referenced are accounted for and we can evaluate this
	// expression.
	return true, ""
}

// gnerateTypeFromValuesMap takes a values map and returns an object type that has the same number of fields, but
// bound to each type of the underlying evaluated expression. This is the only way the HCL decoder will be happy, as
// object type is the only map type that allows different types for each attribute (cty.Map requires all attributes to
// have the same type.
func generateTypeFromValuesMap(valMap map[string]cty.Value) cty.Type {
	outType := map[string]cty.Type{}
	for k, v := range valMap {
		outType[k] = v.Type()
	}
	return cty.Object(outType)
}

// convertValuesMapToCtyVal takes a map of name - cty.Value pairs and converts to a single cty.Value object.
func convertValuesMapToCtyVal(valMap map[string]cty.Value) (cty.Value, error) {
	valMapAsCty := cty.NilVal
	if len(valMap) > 0 {
		var err error
		valMapAsCty, err = gocty.ToCtyValue(valMap, generateTypeFromValuesMap(valMap))
		if err != nil {
			return valMapAsCty, errors.WithStackTrace(err)
		}
	}
	return valMapAsCty, nil
}

// attemptEvaluateLocals attempts to evaluate the locals block given the map of already evaluated locals, replacing
// references to locals with the previously evaluated values. This will return:
// - the list of remaining locals that were unevaluated in this attempt
// - the updated map of evaluated locals after this attempt
// - whether or not any locals were evaluated in this attempt
// - any errors from the evaluation
func attemptEvaluateLocals(
	terragruntOptions *options.TerragruntOptions,
	filename string,
	locals []*config.Local,
	evaluatedLocals map[string]cty.Value,
	contextExtensions *config.EvalContextExtensions,
	diagsWriter hcl.DiagnosticWriter,
) (unevaluatedLocals []*config.Local, newEvaluatedLocals map[string]cty.Value, evaluated bool, err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.WithStackTrace(
				config.PanicWhileParsingConfig{
					RecoveredValue: recovered,
					ConfigFile:     filename,
				},
			)
		}
	}()

	localsAsCtyVal, err := convertValuesMapToCtyVal(evaluatedLocals)
	if err != nil {
		terragruntOptions.Logger.Errorf("Could not convert evaluated locals to the execution context to evaluate additional locals in file %s", filename)
		return nil, evaluatedLocals, false, err
	}
	contextExtensions.Locals = &localsAsCtyVal

	evalCtx, err := CreateTerragruntEvalContext(*contextExtensions, filename, terragruntOptions)
	if err != nil {
		terragruntOptions.Logger.Errorf("Could not convert include to the execution context to evaluate additional locals in file %s", filename)
		return nil, evaluatedLocals, false, err
	}

	evalCtx.Functions[config.FuncNameSopsDecryptFile] = wrapStringSliceToStringAsFuncImpl(NoopSopsDecryptFile, contextExtensions.TrackInclude, terragruntOptions)

	// Track the locals that were evaluated for logging purposes
	newlyEvaluatedLocalNames := []string{}

	unevaluatedLocals = []*config.Local{}
	evaluated = false
	newEvaluatedLocals = map[string]cty.Value{}
	for key, val := range evaluatedLocals {
		newEvaluatedLocals[key] = val
	}
	for _, local := range locals {
		localEvaluated, _ := canEvaluateLocals(local.Expr, evaluatedLocals)
		if localEvaluated {
			evaluatedVal, diags := local.Expr.Value(evalCtx)
			if diags.HasErrors() {
				err := diagsWriter.WriteDiagnostics(diags)
				if err != nil {
					return nil, nil, false, errors.WithStackTrace(err)
				}
				return nil, evaluatedLocals, false, errors.WithStackTrace(diags)
			}
			newEvaluatedLocals[local.Name] = evaluatedVal
			newlyEvaluatedLocalNames = append(newlyEvaluatedLocalNames, local.Name)
			evaluated = true
		} else {
			unevaluatedLocals = append(unevaluatedLocals, local)
		}
	}

	terragruntOptions.Logger.Debugf(
		"Evaluated %d locals (remaining %d): %s",
		len(newlyEvaluatedLocalNames),
		len(unevaluatedLocals),
		strings.Join(newlyEvaluatedLocalNames, ", "),
	)
	return unevaluatedLocals, newEvaluatedLocals, evaluated, nil
}

// decodeLocalsBlock loads the block into name expression pairs to assist with evaluation of the locals prior to
// evaluating the whole config. Note that this is exactly the same as
// terraform/configs/named_values.go:decodeLocalsBlock
func decodeLocalsBlock(localsBlock *hcl.Block) ([]*config.Local, hcl.Diagnostics) {
	attrs, diags := localsBlock.Body.JustAttributes()
	if len(attrs) == 0 {
		return nil, diags
	}

	locals := make([]*config.Local, 0, len(attrs))
	for name, attr := range attrs {
		if !hclsyntax.ValidIdentifier(name) {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid local value name",
				Detail:   badIdentifierDetail,
				Subject:  &attr.NameRange,
			})
		}

		locals = append(locals, &config.Local{
			Name: name,
			Expr: attr.Expr,
		})
	}
	return locals, diags
}

// getBlock takes a parsed HCL file and extracts a reference to the `name` block, if there are defined.
func getBlock(hclFile *hcl.File, name string, isMultipleAllowed bool) ([]*hcl.Block, hcl.Diagnostics) {
	catalogSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: name},
		},
	}
	// We use PartialContent here, because we are only interested in parsing out the catalog block.
	parsed, _, diags := hclFile.Body.PartialContent(catalogSchema)
	extractedBlocks := []*hcl.Block{}
	for _, block := range parsed.Blocks {
		if block.Type == name {
			extractedBlocks = append(extractedBlocks, block)
		}
	}

	if len(extractedBlocks) > 1 && !isMultipleAllowed {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("Multiple %s block", name),
			Detail:   fmt.Sprintf(multipleBlockDetailFmt, name),
		})
		return nil, diags
	}

	return extractedBlocks, diags
}

// evaluateLocalsBlock is a routine to evaluate the locals block in a way to allow references to other locals. This
// will:
//   - Extract a reference to the locals block from the parsed file
//   - Continuously evaluate the block until all references are evaluated, defering evaluation of anything that references
//     other locals until those references are evaluated.
//
// This returns a map of the local names to the evaluated expressions (represented as `cty.Value` objects). This will
// error if there are remaining unevaluated locals after all references that can be evaluated has been evaluated.
func evaluateLocalsBlock(
	terragruntOptions *options.TerragruntOptions,
	parser *hclparse.Parser,
	hclFile *hcl.File,
	filename string,
	contextExtensions *config.EvalContextExtensions,
) (map[string]cty.Value, error) {
	diagsWriter := util.GetDiagnosticsWriter(terragruntOptions.Logger, parser)

	localsBlock, diags := getBlock(hclFile, "locals", false)
	if diags.HasErrors() {
		err := diagsWriter.WriteDiagnostics(diags)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}
		return nil, errors.WithStackTrace(diags)
	}
	if len(localsBlock) == 0 {
		// No locals block referenced in the file
		terragruntOptions.Logger.Debugf("Did not find any locals block: skipping evaluation.")
		return nil, nil
	}

	terragruntOptions.Logger.Debugf("Found locals block: evaluating the expressions.")

	locals, diags := decodeLocalsBlock(localsBlock[0])
	if diags.HasErrors() {
		terragruntOptions.Logger.Errorf("Encountered error while decoding locals block into name expression pairs.")
		err := diagsWriter.WriteDiagnostics(diags)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}
		return nil, errors.WithStackTrace(diags)
	}

	// Continuously attempt to evaluate the locals until there are no more locals to evaluate, or we can't evaluate
	// further.
	evaluatedLocals := map[string]cty.Value{}
	evaluated := true
	for iterations := 0; len(locals) > 0 && evaluated; iterations++ {
		if iterations > config.MaxIter {
			// Reached maximum supported iterations, which is most likely an infinite loop bug so cut the iteration
			// short an return an error.
			return nil, errors.WithStackTrace(config.MaxIterError{})
		}

		var err error
		locals, evaluatedLocals, evaluated, err = attemptEvaluateLocals(
			terragruntOptions,
			filename,
			locals,
			evaluatedLocals,
			contextExtensions,
			diagsWriter,
		)
		if err != nil {
			terragruntOptions.Logger.Errorf("Encountered error while evaluating locals in file %s", filename)
			return nil, err
		}
	}
	if len(locals) > 0 {
		// This is an error because we couldn't evaluate all locals
		terragruntOptions.Logger.Errorf("Not all locals could be evaluated:")
		for _, local := range locals {
			_, reason := canEvaluateLocals(local.Expr, evaluatedLocals)
			terragruntOptions.Logger.Errorf("\t- %s [REASON: %s]", local.Name, reason)
		}
		return nil, errors.WithStackTrace(config.CouldNotEvaluateAllLocalsError{})
	}

	return evaluatedLocals, nil
}

// getTrackInclude converts the terragrunt include blocks into TrackInclude structs that differentiate between an
// included config in the current parsing context, and an included config that was passed through from a previous
// parsing context.
func getTrackInclude(
	terragruntIncludeList []config.IncludeConfig,
	includeFromChild *config.IncludeConfig,
	terragruntOptions *options.TerragruntOptions,
) (*config.TrackInclude, error) {
	includedPaths := []string{}
	terragruntIncludeMap := make(map[string]config.IncludeConfig, len(terragruntIncludeList))
	for _, tgInc := range terragruntIncludeList {
		includedPaths = append(includedPaths, tgInc.Path)
		terragruntIncludeMap[tgInc.Name] = tgInc
	}

	hasInclude := len(terragruntIncludeList) > 0
	var trackInc config.TrackInclude
	switch {
	case hasInclude && includeFromChild != nil:
		// tgInc appears in a parent that is already included, which means a nested include block. This is not
		// something we currently support.
		err := errors.WithStackTrace(config.TooManyLevelsOfInheritance{
			ConfigPath:             terragruntOptions.TerragruntConfigPath,
			FirstLevelIncludePath:  includeFromChild.Path,
			SecondLevelIncludePath: strings.Join(includedPaths, ","),
		})
		return nil, err
	case hasInclude && includeFromChild == nil:
		// Current parsing context where there is no included config already loaded.
		trackInc = config.TrackInclude{
			CurrentList: terragruntIncludeList,
			CurrentMap:  terragruntIncludeMap,
			Original:    nil,
		}
	case !hasInclude:
		// Parsing context where there is an included config already loaded.
		trackInc = config.TrackInclude{
			CurrentList: terragruntIncludeList,
			CurrentMap:  terragruntIncludeMap,
			Original:    includeFromChild,
		}
	}
	return &trackInc, nil
}

// decodeHcl uses the HCL2 parser to decode the parsed HCL into the struct specified by out.
//
// Note that we take a two pass approach to support parsing include blocks without a label. Ideally we can parse include
// blocks with and without labels in a single pass, but the HCL parser is fairly restrictive when it comes to parsing
// blocks with labels, requiring the exact number of expected labels in the parsing step.  To handle this restriction,
// we first see if there are any include blocks without any labels, and if there is, we modify it in the file object to
// inject the label as "".
func decodeHcl2(
	file *hcl.File,
	filename string,
	out interface{},
	evalContext *hcl.EvalContext,
) (err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.WithStackTrace(config.PanicWhileParsingConfig{RecoveredValue: recovered, ConfigFile: filename})
		}
	}()

	// Check if we need to update the file to label any bare include blocks.
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
func decodeAsTerragruntInclude2(
	file *hcl.File,
	filename string,
	evalContext *hcl.EvalContext,
) ([]config.IncludeConfig, error) {
	tgInc := terragruntIncludeMultiple{}
	if err := decodeHcl2(file, filename, &tgInc, evalContext); err != nil {
		return nil, err
	}

	return tgInc.Include, nil
}

func DecodeBaseBlocks(
	terragruntOptions *options.TerragruntOptions,
	parser *hclparse.Parser,
	hclFile *hcl.File,
	filename string,
	includeFromChild *config.IncludeConfig,
	decodeList []config.PartialDecodeSectionType,
) (*config.EvalContextExtensions, error) {
	contextExtensions := &config.EvalContextExtensions{PartialParseDecodeList: decodeList}

	evalContext, err := CreateTerragruntEvalContext(*contextExtensions, filename, terragruntOptions)
	if err != nil {
		return nil, err
	}

	// Decode just the `include` and `import` blocks, and verify that it's allowed here
	terragruntIncludeList, err := decodeAsTerragruntInclude2(
		hclFile,
		filename,
		evalContext,
	)
	if err != nil {
		slog.Error("decodeAsTerragruntInclude2", "err", err)
		return nil, err
	}

	contextExtensions.TrackInclude, err = getTrackInclude(terragruntIncludeList, includeFromChild, terragruntOptions)
	if err != nil {
		return nil, err
	}

	// Evaluate all the expressions in the locals block separately and generate the variables list to use in the
	// evaluation context.
	locals, err := evaluateLocalsBlock(
		terragruntOptions,
		parser,
		hclFile,
		filename,
		contextExtensions,
	)
	if err != nil {
		slog.Error("evaluateLocalsBlock", "err", err)
		return nil, err
	}

	localsAsCtyVal, err := convertValuesMapToCtyVal(locals)
	if err != nil {
		slog.Error("convertValuesMapToCtyVal", "err", err)
		return nil, err
	}
	contextExtensions.Locals = &localsAsCtyVal

	return contextExtensions, nil
}
