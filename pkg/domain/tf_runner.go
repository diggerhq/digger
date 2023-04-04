package domain

type TerraformRunner interface {
	SetWorkingDir(path string) error
	Plan(*PlanOpts) (*TerraformOutput, error)
	Apply(*ApplyOpts) (*TerraformOutput, error)
}

type PlanOpts struct{}
type ApplyOpts struct{}

type TerraformOutput struct {
	Stdout string
	Stderr string
}

func NewTerraformOutput(out string, err string) *TerraformOutput {
	return &TerraformOutput{
		Stdout: out,
		Stderr: err,
	}
}
