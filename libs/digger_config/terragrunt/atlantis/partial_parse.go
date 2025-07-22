package atlantis

import (
	"encoding/json"
	"fmt"
	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/remote"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"path/filepath"
)

// terragruntDependencies is a struct that can be used to only decode the dependencies block.
type terragruntDependencies struct {
	Dependencies *config.ModuleDependencies `hcl:"dependencies,block"`
	Remain       hcl.Body                   `hcl:",remain"`
}

// terragruntTerraform is a struct that can be used to only decode the terraform block
type terragruntTerraform struct {
	Terraform *config.TerraformConfig `hcl:"terraform,block"`
	Remain    hcl.Body                `hcl:",remain"`
}

// terragruntTerraformSource is a struct that can be used to only decode the terraform block, and only the source
// attribute.
type terragruntTerraformSource struct {
	Terraform *terraformConfigSourceOnly `hcl:"terraform,block"`
	Remain    hcl.Body                   `hcl:",remain"`
}

// terragruntDependency is a struct that can be used to only decode the dependency blocks in the terragrunt config
type terragruntDependency struct {
	Dependencies []config.Dependency `hcl:"dependency,block"`
	Remain       hcl.Body            `hcl:",remain"`
}

// terraformConfigSourceOnly is a struct that can be used to decode only the source attribute of the terraform block.
type terraformConfigSourceOnly struct {
	Source *string  `hcl:"source,attr"`
	Remain hcl.Body `hcl:",remain"`
}

// terragruntFlags is a struct that can be used to only decode the flag attributes (skip and prevent_destroy)
type terragruntFlags struct {
	IamRole        *string  `hcl:"iam_role,attr"`
	PreventDestroy *bool    `hcl:"prevent_destroy,attr"`
	Skip           *bool    `hcl:"skip,attr"`
	Remain         hcl.Body `hcl:",remain"`
}

// terragruntVersionConstraints is a struct that can be used to only decode the attributes related to constraining the
// versions of terragrunt and terraform.
type terragruntVersionConstraints struct {
	TerragruntVersionConstraint *string  `hcl:"terragrunt_version_constraint,attr"`
	TerraformVersionConstraint  *string  `hcl:"terraform_version_constraint,attr"`
	TerraformBinary             *string  `hcl:"terraform_binary,attr"`
	Remain                      hcl.Body `hcl:",remain"`
}

type remoteStateConfigGenerate struct {
	// We use cty instead of hcl, since we are using this type to convert an attr and not a block.
	Path     string `cty:"path"`
	IfExists string `cty:"if_exists"`
}

// Configuration for Terraform remote state as parsed from a terragrunt.hcl config file
type remoteStateConfigFile struct {
	Backend                       string                     `hcl:"backend,attr"`
	DisableInit                   *bool                      `hcl:"disable_init,attr"`
	DisableDependencyOptimization *bool                      `hcl:"disable_dependency_optimization,attr"`
	Generate                      *remoteStateConfigGenerate `hcl:"generate,attr"`
	Config                        cty.Value                  `hcl:"config,attr"`
}

func partialParseIncludedConfig(includedConfig *config.IncludeConfig, terragruntOptions *options.TerragruntOptions, decodeList []config.PartialDecodeSectionType) (*config.TerragruntConfig, error) {
	if includedConfig.Path == "" {
		return nil, errors.WithStackTrace(config.IncludedConfigMissingPath(terragruntOptions.TerragruntConfigPath))
	}

	includePath := includedConfig.Path

	if !filepath.IsAbs(includePath) {
		includePath = util.JoinPath(filepath.Dir(terragruntOptions.TerragruntConfigPath), includePath)
	}

	return PartialParseConfigFile(
		includePath,
		terragruntOptions,
		includedConfig,
		decodeList,
	)
}

// handleIncludePartial merges the a partially parsed include config into the child config according to the strategy
// specified by the user.
func handleIncludePartial(
	config2 *config.TerragruntConfig,
	trackInclude *config.TrackInclude,
	terragruntOptions *options.TerragruntOptions,
	decodeList []config.PartialDecodeSectionType,
) (*config.TerragruntConfig, error) {
	if trackInclude == nil {
		return nil, fmt.Errorf("You reached an impossible condition. This is most likely a bug in terragrunt. Please open an issue at github.com/gruntwork-io/terragrunt with this error message. Code: HANDLE_INCLUDE_PARTIAL_NIL_INCLUDE_CONFIG")
	}

	// We merge in the include blocks in reverse order here. The expectation is that the bottom most elements override
	// those in earlier includes, so we need to merge bottom up instead of top down to ensure this.
	includeList := trackInclude.CurrentList
	baseConfig := config2
	for i := len(includeList) - 1; i >= 0; i-- {
		includeConfig := includeList[i]
		mergeStrategy, err := includeConfig.GetMergeStrategy()
		if err != nil {
			return nil, err
		}

		parsedIncludeConfig, err := partialParseIncludedConfig(&includeConfig, terragruntOptions, decodeList)
		if err != nil {
			return nil, err
		}

		switch mergeStrategy {
		case config.NoMerge:
			terragruntOptions.Logger.Debugf("[Partial] Included config %s has strategy no merge: not merging config in.", includeConfig.Path)
		case config.ShallowMerge:
			terragruntOptions.Logger.Debugf("[Partial] Included config %s has strategy shallow merge: merging config in (shallow).", includeConfig.Path)
			if err := parsedIncludeConfig.Merge(baseConfig, terragruntOptions); err != nil {
				return nil, err
			}
			baseConfig = parsedIncludeConfig
		case config.DeepMerge:
			terragruntOptions.Logger.Debugf("[Partial] Included config %s has strategy deep merge: merging config in (deep).", includeConfig.Path)
			if err := parsedIncludeConfig.DeepMerge(baseConfig, terragruntOptions); err != nil {
				return nil, err
			}
			baseConfig = parsedIncludeConfig
		default:
			return nil, fmt.Errorf("You reached an impossible condition. This is most likely a bug in terragrunt. Please open an issue at github.com/gruntwork-io/terragrunt with this error message. Code: UNKNOWN_MERGE_STRATEGY_%s_PARTIAL", mergeStrategy)
		}
	}
	return baseConfig, nil
}

// Convert the parsed config file remote state struct to the internal representation struct of remote state
// configurations.
func (remoteState *remoteStateConfigFile) toConfig() (*remote.RemoteState, error) {
	remoteStateConfig, err := parseCtyValueToMap(remoteState.Config)
	if err != nil {
		return nil, err
	}

	config := &remote.RemoteState{}
	config.Backend = remoteState.Backend
	if remoteState.Generate != nil {
		config.Generate = &remote.RemoteStateGenerate{
			Path:     remoteState.Generate.Path,
			IfExists: remoteState.Generate.IfExists,
		}
	}
	config.Config = remoteStateConfig

	if remoteState.DisableInit != nil {
		config.DisableInit = *remoteState.DisableInit
	}
	if remoteState.DisableDependencyOptimization != nil {
		config.DisableDependencyOptimization = *remoteState.DisableDependencyOptimization
	}

	config.FillDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, err
}

// terragruntRemoteState is a struct that can be used to only decode the remote_state blocks in the terragrunt config
type terragruntRemoteState struct {
	RemoteState *remoteStateConfigFile `hcl:"remote_state,block"`
	Remain      hcl.Body               `hcl:",remain"`
}

type InvalidPartialBlockName struct {
	sectionCode config.PartialDecodeSectionType
}

func (err InvalidPartialBlockName) Error() string {
	return fmt.Sprintf("Unrecognized partial block code %d. This is most likely an error in terragrunt. Please file a bug report on the project repository.", err.sectionCode)
}

func isEnabled(dependencyConfig config.Dependency) bool {
	if dependencyConfig.Enabled == nil {
		return true
	}
	return *dependencyConfig.Enabled
}

// Convert the list of parsed Dependency blocks into a list of module dependencies. Each output block should
// become a dependency of the current config, since that module has to be applied before we can read the output.
func dependencyBlocksToModuleDependencies(decodedDependencyBlocks []config.Dependency) *config.ModuleDependencies {
	if len(decodedDependencyBlocks) == 0 {
		return nil
	}

	paths := []string{}
	for _, decodedDependencyBlock := range decodedDependencyBlocks {
		// skip dependency if is not enabled
		if !isEnabled(decodedDependencyBlock) {
			continue
		}
		paths = append(paths, decodedDependencyBlock.ConfigPath)
	}

	return &config.ModuleDependencies{Paths: paths}
}

// This is a hacky workaround to convert a cty Value to a Go map[string]interface{}. cty does not support this directly
// (https://github.com/hashicorp/hcl2/issues/108) and doing it with gocty.FromCtyValue is nearly impossible, as cty
// requires you to specify all the output types and will error out when it hits interface{}. So, as an ugly workaround,
// we convert the given value to JSON using cty's JSON library and then convert the JSON back to a
// map[string]interface{} using the Go json library.
func parseCtyValueToMap(value cty.Value) (map[string]interface{}, error) {
	jsonBytes, err := ctyjson.Marshal(value, cty.DynamicPseudoType)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	var ctyJsonOutput config.CtyJsonOutput
	if err := json.Unmarshal(jsonBytes, &ctyJsonOutput); err != nil {
		return nil, errors.WithStackTrace(err)
	}

	return ctyJsonOutput.Value, nil
}

// PartialParseConfigString partially parses and decodes the provided string. Which blocks/attributes to decode is
// controlled by the function parameter decodeList. These blocks/attributes are parsed and set on the output
// TerragruntConfig. Valid values are:
//   - DependenciesBlock: Parses the `dependencies` block in the config
//   - DependencyBlock: Parses the `dependency` block in the config
//   - TerraformBlock: Parses the `terraform` block in the config
//   - TerragruntFlags: Parses the boolean flags `prevent_destroy` and `skip` in the config
//   - TerragruntVersionConstraints: Parses the attributes related to constraining terragrunt and terraform versions in
//     the config.
//   - RemoteStateBlock: Parses the `remote_state` block in the config
//
// Note that the following blocks are always decoded:
// - locals
// - include
// Note also that the following blocks are never decoded in a partial parse:
// - inputs
func PartialParseConfigString(
	configString string,
	terragruntOptions *options.TerragruntOptions,
	includeFromChild *config.IncludeConfig,
	filename string,
	decodeList []config.PartialDecodeSectionType,
) (*config.TerragruntConfig, error) {
	// Parse the HCL string into an AST body that can be decoded multiple times later without having to re-parse
	parser := hclparse.NewParser()
	file, err := parseHcl(parser, configString, filename)
	if err != nil {
		return nil, err
	}

	// Decode just the Base blocks. See the function docs for DecodeBaseBlocks for more info on what base blocks are.
	// Initialize evaluation context extensions from base blocks.
	contextExtensions, err := DecodeBaseBlocks(terragruntOptions, parser, file, filename, includeFromChild, decodeList)
	if err != nil {
		return nil, err
	}

	output := config.TerragruntConfig{IsPartial: true}

	// Set parsed Locals on the parsed config
	if contextExtensions.Locals != nil && *contextExtensions.Locals != cty.NilVal {
		localsParsed, err := parseCtyValueToMap(*contextExtensions.Locals)
		if err != nil {
			return nil, err
		}
		output.Locals = localsParsed
	}

	evalContext, err := CreateTerragruntEvalContext(*contextExtensions, filename, terragruntOptions)
	if err != nil {
		return nil, err
	}

	// Now loop through each requested block / component to decode from the terragrunt config, decode them, and merge
	// them into the output TerragruntConfig struct.
	for _, decode := range decodeList {
		switch decode {
		case config.DependenciesBlock:
			decoded := terragruntDependencies{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}

			// If we already decoded some dependencies, merge them in. Otherwise, set as the new list.
			if output.Dependencies != nil {
				output.Dependencies.Merge(decoded.Dependencies)
			} else {
				output.Dependencies = decoded.Dependencies
			}

		case config.TerraformBlock:
			decoded := terragruntTerraform{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}
			output.Terraform = decoded.Terraform

		case config.TerraformSource:
			decoded := terragruntTerraformSource{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}
			if decoded.Terraform != nil {
				output.Terraform = &config.TerraformConfig{Source: decoded.Terraform.Source}
			}

		case config.DependencyBlock:
			decoded := terragruntDependency{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}
			output.TerragruntDependencies = decoded.Dependencies

			// Convert dependency blocks into module depenency lists. If we already decoded some dependencies,
			// merge them in. Otherwise, set as the new list.
			dependencies := dependencyBlocksToModuleDependencies(decoded.Dependencies)
			if output.Dependencies != nil {
				output.Dependencies.Merge(dependencies)
			} else {
				output.Dependencies = dependencies
			}

		case config.TerragruntFlags:
			decoded := terragruntFlags{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}
			if decoded.PreventDestroy != nil {
				output.PreventDestroy = decoded.PreventDestroy
			}
			if decoded.Skip != nil {
				output.Skip = *decoded.Skip
			}
			if decoded.IamRole != nil {
				output.IamRole = *decoded.IamRole
			}

		case config.TerragruntVersionConstraints:
			decoded := terragruntVersionConstraints{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}
			if decoded.TerragruntVersionConstraint != nil {
				output.TerragruntVersionConstraint = *decoded.TerragruntVersionConstraint
			}
			if decoded.TerraformVersionConstraint != nil {
				output.TerraformVersionConstraint = *decoded.TerraformVersionConstraint
			}
			if decoded.TerraformBinary != nil {
				output.TerraformBinary = *decoded.TerraformBinary
			}

		case config.RemoteStateBlock:
			decoded := terragruntRemoteState{}
			err := decodeHcl2(file, filename, &decoded, evalContext)
			if err != nil {
				return nil, err
			}
			if decoded.RemoteState != nil {
				remoteState, err := decoded.RemoteState.toConfig()
				if err != nil {
					return nil, err
				}
				output.RemoteState = remoteState
			}

		default:
			return nil, InvalidPartialBlockName{decode}
		}
	}

	// If this file includes another, parse and merge the partial blocks.  Otherwise just return this config.
	if len(contextExtensions.TrackInclude.CurrentList) > 0 {
		config, err := handleIncludePartial(&output, contextExtensions.TrackInclude, terragruntOptions, decodeList)
		if err != nil {
			return nil, err
		}
		// Saving processed includes into configuration, direct assignment since nested includes aren't supported
		config.ProcessedIncludes = contextExtensions.TrackInclude.CurrentMap
		return config, nil
	}
	return &output, nil
}

var terragruntConfigCache = config.NewTerragruntConfigCache()

func TerragruntConfigFromPartialConfigString(
	configString string,
	terragruntOptions *options.TerragruntOptions,
	includeFromChild *config.IncludeConfig,
	filename string,
	decodeList []config.PartialDecodeSectionType,
) (*config.TerragruntConfig, error) {
	if terragruntOptions.UsePartialParseConfigCache {
		var cacheKey = fmt.Sprintf("%#v-%#v-%#v-%#v", filename, configString, includeFromChild, decodeList)
		var config, found = terragruntConfigCache.Get(cacheKey)

		if !found {
			terragruntOptions.Logger.Debugf("Cache miss for '%s' (partial parsing), decodeList: '%v'.", filename, decodeList)
			tgConfig, err := PartialParseConfigString(configString, terragruntOptions, includeFromChild, filename, decodeList)
			if err != nil {
				return nil, err
			}
			config = *tgConfig
			terragruntConfigCache.Put(cacheKey, config)
		} else {
			terragruntOptions.Logger.Debugf("Cache hit for '%s' (partial parsing), decodeList: '%v'.", filename, decodeList)
		}

		return &config, nil
	} else {
		return PartialParseConfigString(configString, terragruntOptions, includeFromChild, filename, decodeList)
	}
}

func PartialParseConfigFile(
	filename string,
	terragruntOptions *options.TerragruntOptions,
	include *config.IncludeConfig,
	decodeList []config.PartialDecodeSectionType,
) (*config.TerragruntConfig, error) {
	configString, err := util.ReadFileAsString(filename)
	if err != nil {
		return nil, err
	}

	config, err := TerragruntConfigFromPartialConfigString(configString, terragruntOptions, include, filename, decodeList)
	if err != nil {
		return nil, err
	}

	return config, nil
}
