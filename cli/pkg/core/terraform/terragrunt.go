package terraform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Terragrunt struct {
	WorkingDir string
}

func (terragrunt Terragrunt) Init(params []string, envs map[string]string) (string, string, error) {
	return terragrunt.runTerragruntCommand("init", envs, params...)

}

func (terragrunt Terragrunt) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	params = append(params, "--auto-approve")
	params = append(params, "--terragrunt-non-interactive")
	if plan != nil {
		params = append(params, *plan)
	}
	stdout, stderr, err := terragrunt.runTerragruntCommand("apply", envs, params...)
	return stdout, stderr, err
}

func (terragrunt Terragrunt) Destroy(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "--auto-approve")
	params = append(params, "--terragrunt-non-interactive")
	stdout, stderr, err := terragrunt.runTerragruntCommand("destroy", envs, params...)
	return stdout, stderr, err
}

func (terragrunt Terragrunt) Plan(params []string, envs map[string]string) (bool, string, string, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("plan", envs, params...)
	return true, stdout, stderr, err
}

func (terragrunt Terragrunt) Show(params []string, envs map[string]string) (string, string, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("show", envs, params...)
	return stdout, stderr, err
}

func (terragrunt Terragrunt) runTerragruntCommand(command string, envs map[string]string, arg ...string) (string, string, error) {
	args := []string{command}
	args = append(args, arg...)
	cmd := exec.Command("terragrunt", args...)
	cmd.Dir = terragrunt.WorkingDir

	env := os.Environ()
	env = append(env, "TF_CLI_ARGS=-no-color")
	env = append(env, "TF_IN_AUTOMATION=true")

	for k, v := range envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

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
