package execution

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Terragrunt struct {
	WorkingDir string
}

func (terragrunt Terragrunt) Init(params []string, envs map[string]string) (string, string, error) {

	stdout, stderr, exitCode, err := terragrunt.runTerragruntCommand("init", true, envs, params...)
	if exitCode != 0 {
		logCommandFail(exitCode, err)
	}

	return stdout, stderr, err
}

func (terragrunt Terragrunt) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	params = append(params, []string{"-lock-timeout=3m"}...)
	params = append(params, "--auto-approve")
	params = append(params, "--terragrunt-non-interactive")
	if plan != nil {
		params = append(params, *plan)
	}
	stdout, stderr, exitCode, err := terragrunt.runTerragruntCommand("apply", true, envs, params...)
	if exitCode != 0 {
		logCommandFail(exitCode, err)
	}

	return stdout, stderr, err
}

func (terragrunt Terragrunt) Destroy(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "--auto-approve")
	params = append(params, "--terragrunt-non-interactive")
	stdout, stderr, exitCode, err := terragrunt.runTerragruntCommand("destroy", true, envs, params...)
	if exitCode != 0 {
		logCommandFail(exitCode, err)
	}

	return stdout, stderr, err
}

func (terragrunt Terragrunt) Plan(params []string, envs map[string]string, planArtefactFilePath string) (bool, string, string, error) {
	if planArtefactFilePath != "" {
		params = append(params, []string{"-out", planArtefactFilePath}...)
	}
	params = append(params, "-lock-timeout=3m")
	stdout, stderr, exitCode, err := terragrunt.runTerragruntCommand("plan", true, envs, params...)
	if exitCode != 0 {
		logCommandFail(exitCode, err)
	}

	return true, stdout, stderr, err
}

func (terragrunt Terragrunt) Show(params []string, envs map[string]string, planArtefactFilePath string) (string, string, error) {
	params = append(params, []string{"-no-color", "-json", planArtefactFilePath}...)
	stdout, stderr, exitCode, err := terragrunt.runTerragruntCommand("show", false, envs, params...)
	if exitCode != 0 {
		logCommandFail(exitCode, err)
	}

	return stdout, stderr, err
}

func (terragrunt Terragrunt) runTerragruntCommand(command string, printOutputToStdout bool, envs map[string]string, arg ...string) (stdOut string, stdErr string, exitCode int, err error) {
	args := []string{command}
	args = append(args, arg...)

	expandedArgs := make([]string, 0)
	for _, p := range args {
		s := os.ExpandEnv(p)
		s = strings.TrimSpace(s)
		if s != "" {
			expandedArgs = append(expandedArgs, s)
		}
	}

	// Set up common output buffers
	var mwout, mwerr io.Writer
	var stdout, stderr bytes.Buffer
	if printOutputToStdout {
		mwout = io.MultiWriter(os.Stdout, &stdout)
		mwerr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		mwout = io.Writer(&stdout)
		mwerr = io.Writer(&stderr)
	}

	cmd := exec.Command("terragrunt", args...)
	log.Printf("Running command: terragrunt %v", expandedArgs)
	cmd.Dir = terragrunt.WorkingDir

	env := os.Environ()
	env = append(env, "TF_CLI_ARGS=-no-color")
	env = append(env, "TF_IN_AUTOMATION=true")

	for k, v := range envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Env = env
	cmd.Stdout = mwout
	cmd.Stderr = mwerr

	err = cmd.Run()
	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode(), err
}
