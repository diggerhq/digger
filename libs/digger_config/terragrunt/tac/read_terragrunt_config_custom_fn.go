package tac

import (
	"path/filepath"

	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var safeReadTerragruntConfigDecodeList = []config.PartialDecodeSectionType{
	config.DependenciesBlock,
	config.DependencyBlock,
	config.TerraformBlock,
	config.TerragruntFlags,
	config.TerragruntVersionConstraints,
	config.RemoteStateBlock,
	partialDecodeInputs,
}

func wrapVoidToStringAsFuncImpl(
	toWrap func(trackInclude *config.TrackInclude, terragruntOptions *options.TerragruntOptions) (string, error),
	trackInclude *config.TrackInclude,
	terragruntOptions *options.TerragruntOptions,
) function.Function {
	return function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			out, err := toWrap(trackInclude, terragruntOptions)
			if err != nil {
				return cty.StringVal(""), err
			}
			return cty.StringVal(out), nil
		},
	})
}

func NoopAWSIdentity(trackInclude *config.TrackInclude, terragruntOptions *options.TerragruntOptions) (string, error) {
	terragruntOptions.Logger.Debugf("AWS identity helper has been replaced with a no-op version. This is to ensure that generation of projects is successful.")
	return "", nil
}

// readTerragruntConfig uses the local partial parser so nested reads inherit the hardened eval context.
func readTerragruntConfig(configPath string, defaultVal *cty.Value, terragruntOptions *options.TerragruntOptions) (cty.Value, error) {
	targetConfig := getCleanedTargetConfigPath(configPath, terragruntOptions.TerragruntConfigPath)
	targetConfigFileExists := util.FileExists(targetConfig)
	if !targetConfigFileExists && defaultVal == nil {
		return cty.NilVal, errors.WithStackTrace(config.TerragruntConfigNotFound{Path: targetConfig})
	} else if !targetConfigFileExists {
		return *defaultVal, nil
	}

	targetOptions := terragruntOptions.Clone(targetConfig)
	parsedConfig, err := PartialParseConfigFile(targetConfig, targetOptions, nil, safeReadTerragruntConfigDecodeList)
	if err != nil {
		return cty.NilVal, err
	}

	return config.TerragruntConfigAsCty(parsedConfig)
}

func readTerragruntConfigAsFuncImpl(terragruntOptions *options.TerragruntOptions) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{{Type: cty.String}},
		VarParam: &function.Parameter{
			Type: cty.DynamicPseudoType,
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			numParams := len(args)
			if numParams == 0 || numParams > 2 {
				return cty.NilVal, errors.WithStackTrace(config.WrongNumberOfParams{Func: "read_terragrunt_config", Expected: "1 or 2", Actual: numParams})
			}

			strArgs, err := ctySliceToStringSlice(args[:1])
			if err != nil {
				return cty.NilVal, err
			}

			var defaultVal *cty.Value
			if numParams == 2 {
				defaultVal = &args[1]
			}

			return readTerragruntConfig(strArgs[0], defaultVal, terragruntOptions)
		},
	})
}

func getCleanedTargetConfigPath(configPath string, workingPath string) string {
	cwd := filepath.Dir(workingPath)
	targetConfig := configPath
	if !filepath.IsAbs(targetConfig) {
		targetConfig = util.JoinPath(cwd, targetConfig)
	}
	if util.IsDir(targetConfig) {
		targetConfig = config.GetDefaultConfigPath(targetConfig)
	}
	return util.CleanPath(targetConfig)
}
