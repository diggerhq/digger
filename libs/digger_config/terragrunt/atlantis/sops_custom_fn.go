package atlantis

import (
	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func ctySliceToStringSlice(args []cty.Value) ([]string, error) {
	var out []string
	for _, arg := range args {
		if arg.Type() != cty.String {
			return nil, errors.WithStackTrace(config.InvalidParameterType{Expected: "string", Actual: arg.Type().FriendlyName()})
		}
		out = append(out, arg.AsString())
	}
	return out, nil
}

func wrapStringSliceToStringAsFuncImpl(
	toWrap func(params []string, trackInclude *config.TrackInclude, terragruntOptions *options.TerragruntOptions) (string, error),
	trackInclude *config.TrackInclude,
	terragruntOptions *options.TerragruntOptions,
) function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{Type: cty.String},
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			params, err := ctySliceToStringSlice(args)
			if err != nil {
				return cty.StringVal(""), err
			}
			out, err := toWrap(params, trackInclude, terragruntOptions)
			if err != nil {
				return cty.StringVal(""), err
			}
			return cty.StringVal(out), nil
		},
	})
}

func NoopSopsDecryptFile(params []string, trackInclude *config.TrackInclude, terragruntOptions *options.TerragruntOptions) (string, error) {
	terragruntOptions.Logger.Debugf("SOPS decryption function has been replaced with a no-op version. This is to ensure that generation of projects is successful.")
	return "{}", nil
}
