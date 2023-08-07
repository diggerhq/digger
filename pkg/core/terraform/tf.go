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
	Init([]string, map[string]string) (string, string, error)
	Apply([]string, *string, map[string]string) (string, string, error)
	Plan([]string, map[string]string) (bool, string, string, error)
	Show([]string, map[string]string) (string, string, error)
}

type Terragrunt struct {
	WorkingDir string
}

type Terraform struct {
	WorkingDir string
	Workspace  string
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

	terragruntCommandLine := "terragrunt "
	for _, p := range args {
		trimmedArg := strings.TrimSpace(p)
		if trimmedArg != "" {
			terragruntCommandLine += trimmedArg + " "
		}
	}
	fmt.Println("terragrunt command:")
	fmt.Println(terragruntCommandLine)

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

func (tf Terraform) Init(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "-upgrade=true")
	params = append(params, "-input=false")
	params = append(params, "-no-color")
	stdout, stderr, _, err := tf.runTerraformCommand("init", envs, params...)
	return stdout, stderr, err
}

func (tf Terraform) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	workspaces, _, _, err := tf.runTerraformCommand("workspace", envs, "list")
	if err != nil {
		return "", "", err
	}
	workspaces = tf.formatTerraformWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		_, _, _, err := tf.runTerraformCommand("workspace", envs, "select", tf.Workspace)
		if err != nil {
			return "", "", err
		}
	} else {
		_, _, _, err := tf.runTerraformCommand("workspace", envs, "new", tf.Workspace)
		if err != nil {
			return "", "", err
		}
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	if plan != nil {
		params = append(params, *plan)
	}
	stdout, stderr, _, err := tf.runTerraformCommand("apply", envs, params...)
	return stdout, stderr, err
}

func (tf Terraform) runTerraformCommand(command string, envs map[string]string, arg ...string) (string, string, int, error) {
	args := []string{command}
	args = append(args, arg...)

	expandedArgs := make([]string, 0)
	terraformCommandLine := "terraform "
	for _, p := range args {
		s := os.ExpandEnv(p)
		s = strings.TrimSpace(s)
		if s != "" {
			expandedArgs = append(expandedArgs, s)
			terraformCommandLine += s + " "
		}
	}
	fmt.Println("terraform command:")
	fmt.Println(terraformCommandLine)

	var stdout, stderr bytes.Buffer
	mwout := io.MultiWriter(os.Stdout, &stdout)
	mwerr := io.MultiWriter(os.Stderr, &stderr)

	cmd := exec.Command("terraform", expandedArgs...)
	cmd.Dir = tf.WorkingDir

	env := os.Environ()
	for k, v := range envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
	cmd.Stdout = mwout
	cmd.Stderr = mwerr

	err := cmd.Run()

	// terraform plan can return 2 if there are changes to be applied, so we don't want to fail in that case
	if err != nil && cmd.ProcessState.ExitCode() != 2 {
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

func (tf Terraform) formatTerraformWorkspaces(list string) string {

	list = strings.TrimSpace(list)
	char_replace := strings.NewReplacer("*", "", "\n", ",", " ", "")
	list = char_replace.Replace(list)
	return list
}

func (tf Terraform) Plan(params []string, envs map[string]string) (bool, string, string, error) {

	workspaces, _, _, err := tf.runTerraformCommand("workspace", envs, "list")
	if err != nil {
		return false, "", "", err
	}
	workspaces = tf.formatTerraformWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		_, _, _, err := tf.runTerraformCommand("workspace", envs, "select", tf.Workspace)
		if err != nil {
			return false, "", "", err
		}
	} else {
		_, _, _, err := tf.runTerraformCommand("workspace", envs, "new", tf.Workspace)
		if err != nil {
			return false, "", "", err
		}
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-detailed-exitcode")
	stdout, stderr, statusCode, err := tf.runTerraformCommand("plan", envs, params...)
	if err != nil && statusCode != 2 {
		return false, "", "", err
	}
	return statusCode == 2, stdout, stderr, nil
}

func (tf Terraform) Show(params []string, envs map[string]string) (string, string, error) {
	stdout, stderr, _, err := tf.runTerraformCommand("show", envs, params...)
	if err != nil {
		return "", "", err
	}
	return stdout, stderr, nil
}
