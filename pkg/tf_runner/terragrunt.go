package tf_runner

import (
	"bytes"
	"digger/pkg/domain"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Terragrunt struct {
	WorkingDir string
}

func (terragrunt *Terragrunt) SetWorkingDir(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	terragrunt.WorkingDir = path
	return nil
}

func (terragrunt *Terragrunt) Apply(applyOpts *domain.ApplyOpts) (*domain.TerraformOutput, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("apply")
	return domain.NewTerraformOutput(stdout, stderr), err
}

func (terragrunt *Terragrunt) Plan(planOpts *domain.PlanOpts) (*domain.TerraformOutput, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("plan")
	return domain.NewTerraformOutput(stdout, stderr), err
}

func (terragrunt *Terragrunt) runTerragruntCommand(command string) (string, string, error) {
	cmd := exec.Command("terragrunt", command, "--terragrunt-working-dir", terragrunt.WorkingDir)
	env := os.Environ()
	env = append(env, "TF_CLI_ARGS=-no-color")
	env = append(env, "TF_IN_AUTOMATION=true")
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	mwout := io.MultiWriter(os.Stdout, &stdout)
	mwerr := io.MultiWriter(os.Stderr, &stderr)
	cmd.Stdout = mwout
	cmd.Stderr = mwerr
	err := cmd.Run()

	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("error: %v", err)
	}

	return stdout.String(), stderr.String(), err
}
