package atlantis

import (
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/hashicorp/hcl/v2"
)

// Wrapper around the config.CreateTerragruntEvalContext function to override the sops_decrypt_file function
func CreateTerragruntEvalContext(extensions config.EvalContextExtensions, filename string, terragruntOptions *options.TerragruntOptions) (*hcl.EvalContext, error) {
	ctx, err := extensions.CreateTerragruntEvalContext(filename, terragruntOptions)
	if err != nil {
		return ctx, err
	}

	// override sops_decrypt_file function
	ctx.Functions[config.FuncNameSopsDecryptFile] = wrapStringSliceToStringAsFuncImpl(NoopSopsDecryptFile, extensions.TrackInclude, terragruntOptions)
	return ctx, nil
}
