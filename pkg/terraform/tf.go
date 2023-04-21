package terraform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type TerraformExecutor interface {
	Init([]string) (string, string, error)
	Apply([]string) (string, string, error)
	Plan([]string) (bool, string, string, error)
}

type Terragrunt struct {
	WorkingDir string
}

type Terraform struct {
	WorkingDir string
	Workspace  string
}

func (terragrunt Terragrunt) Init(params []string) (string, string, error) {
	return terragrunt.runTerragruntCommand("init", params...)

}

func (terragrunt Terragrunt) Apply(params []string) (string, string, error) {
	params = append(params, "--auto-approve")
	params = append(params, "--terragrunt-non-interactive")
	stdout, stderr, err := terragrunt.runTerragruntCommand("apply", params...)
	return stdout, stderr, err
}

func (terragrunt Terragrunt) Plan(params []string) (bool, string, string, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("plan", params...)
	return true, stdout, stderr, err
}

func (terragrunt Terragrunt) runTerragruntCommand(command string, arg ...string) (string, string, error) {
	args := []string{command}
	args = append(args, arg...)
	args = append(args, "--terragrunt-working-dir", terragrunt.WorkingDir)
	cmd := exec.Command("terragrunt", args...)

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

func (tf Terraform) Init(params []string) (string, string, error) {
	params = append(params, "-upgrade=true")
	params = append(params, "-input=false")
	params = append(params, "-no-color")
	stdout, stderr, _, err := tf.runTerraformCommand("init", params...)
	return stdout, stderr, err
}

func (tf Terraform) Apply(params []string) (string, string, error) {
	workspace, _, _, err := tf.runTerraformCommand("workspace", "show")

	if err != nil {
		return "", "", err
	}

	if strings.TrimSpace(workspace) != tf.Workspace {
		_, _, _, err = tf.runTerraformCommand("workspace", "new", tf.Workspace)
		if err != nil {
			return "", "", err
		}
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	stdout, stderr, _, err := tf.runTerraformCommand("apply", params...)
	if err != nil {
		return "", "", err
	}
	return stdout, stderr, nil
}

func (tf Terraform) runTerraformCommand(command string, arg ...string) (string, string, int, error) {
	args := []string{"-chdir=" + tf.WorkingDir}
	args = append(args, command)
	args = append(args, arg...)
	fmt.Printf("Running terraform %v", args)

	var stdout, stderr bytes.Buffer
	mwout := io.MultiWriter(os.Stdout, &stdout)
	mwerr := io.MultiWriter(os.Stderr, &stderr)

	cmd := exec.Command("terraform", args...)

	cmd.Stdout = mwout
	cmd.Stderr = mwerr

	err := cmd.Run()

	if err != nil {
		fmt.Println("Error:", err)
	}

	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode(), err
}

type StdWriter struct {
	data  []byte
	print bool
}

func (sw *StdWriter) Write(data []byte) (n int, err error) {
	s := string(data)
	if sw.print {
		print(s)
	}

	sw.data = append(sw.data, data...)
	return 0, nil
}

func (sw *StdWriter) GetString() string {
	s := string(sw.data)
	return s
}

func (tf Terraform) Plan(params []string) (bool, string, string, error) {
	workspace, _, _, err := tf.runTerraformCommand("workspace", "show")

	if err != nil {
		return false, "", "", err
	}
	if strings.TrimSpace(workspace) != tf.Workspace {
		_, _, _, err = tf.runTerraformCommand("workspace", "new", tf.Workspace)
		if err != nil {
			return false, "", "", err
		}
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-out=tfplan")
	stdout, stderr, statusCode, err := tf.runTerraformCommand("plan", params...)
	if err != nil {
		return false, "", "", err
	}
	return statusCode != 2, stdout, stderr, nil
}
