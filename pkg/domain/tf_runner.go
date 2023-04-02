package domain

type TerraformRunner interface {
	SetWorkingDir(path string) error
	Plan() (*TerraformOutput, error)
	Apply() (*TerraformOutput, error)
}

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
