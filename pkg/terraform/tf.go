package terraform

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-exec/tfexec"
	"log"
	"os"
	"os/exec"
)

type TerraformExecutor interface {
	Apply([]string, []string) (string, string, error)
	Plan([]string, []string) (bool, string, string, error)
}

type Terragrunt struct {
	WorkingDir string
}

type Terraform struct {
	WorkingDir string
	Workspace  string
}

func (terragrunt Terragrunt) Apply(initParams []string, applyParams []string) (string, string, error) {
	return terragrunt.runTerragruntCommand("apply")
}

func (terragrunt Terragrunt) Plan(initParams []string, planParams []string) (bool, string, string, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("plan")
	return true, stdout, stderr, err
}

func (terragrunt Terragrunt) runTerragruntCommand(command string) (string, string, error) {
	cmd := exec.Command("terragrunt", command, "--terragrunt-working-dir", terragrunt.WorkingDir)

	stdout := StdWriter{[]byte{}, true}
	stderr := StdWriter{[]byte{}, true}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		fmt.Println("Error:", err)
	}

	return stdout.GetString(), stderr.GetString(), err
}

func (terraform Terraform) Apply(initParams []string, applyParams []string) (string, string, error) {
	initParams = append(append(initParams, "-upgrade=false"), "-input=false")
	_, _, err := terraform.runTerraformCommand("init", initParams...)
	if err != nil {
		return "", "", err
	}
	_, _, err = terraform.runTerraformCommand("select", terraform.Workspace)
	if err != nil {
		return "", "", err
	}
	return terraform.runTerraformCommand("apply", applyParams...)
}

func (tf Terraform) runTerraformCommand(command string, arg ...string) (string, string, error) {
	args := []string{command}
	args = append(args, arg...)
	args = append(args, "-chdir="+tf.WorkingDir)

	cmd := exec.Command("terraform", args...)

	stdout := StdWriter{[]byte{}, true}
	stderr := StdWriter{[]byte{}, true}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		fmt.Println("Error:", err)
	}

	return stdout.GetString(), stderr.GetString(), err
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

func (terraform Terraform) Plan(initParams []string, planParams []string) (bool, string, string, error) {
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.WorkingDir, execDir)

	if err != nil {
		println("Error while initializing terraform: " + err.Error())
		os.Exit(1)
	}
	stdout := &StdWriter{[]byte{}, true}
	stderr := &StdWriter{[]byte{}, true}
	tf.SetStdout(stdout)
	tf.SetStderr(stderr)

	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		println("terraform init failed.")
		return false, stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform init failed. %s", err)
	}
	err = tf.WorkspaceSelect(context.Background(), terraform.Workspace)

	if err != nil {
		log.Printf("terraform workspace select failed. workspace: %v. dir: %v", terraform.Workspace, terraform.WorkingDir)
		return false, stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform select failed. %s", err)
	}
	isNonEmptyPlan, err := tf.Plan(context.Background())
	if err != nil {
		println("terraform plan failed. dir: " + terraform.WorkingDir)
		return isNonEmptyPlan, stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform plan failed. %s", err)
	}

	return isNonEmptyPlan, stdout.GetString(), stderr.GetString(), nil
}
