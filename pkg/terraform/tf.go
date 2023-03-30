package terraform

import (
	"bytes"
	"context"
	"fmt"
	"github.com/hashicorp/terraform-exec/tfexec"
	"log"
	"os"
	"os/exec"
)

type TerraformExecutor interface {
	Apply() (string, string, error)
	Plan() (bool, string, string, error)
}

type Terragrunt struct {
	WorkingDir string
}

type Terraform struct {
	WorkingDir string
	Workspace  string
}

func (terragrunt Terragrunt) Apply() (string, string, error) {
	return terragrunt.runTerragruntCommand("apply")
}

func (terragrunt Terragrunt) Plan() (bool, string, string, error) {
	stdout, stderr, err := terragrunt.runTerragruntCommand("plan")
	return true, stdout, stderr, err
}

func (terragrunt Terragrunt) runTerragruntCommand(command string) (string, string, error) {
	cmd := exec.Command("terragrunt", command, "--terragrunt-working-dir", terragrunt.WorkingDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		fmt.Println("Error:", err)
	}

	return stdout.String(), stderr.String(), err
}

func (terraform Terraform) Apply() (string, string, error) {
	println("digger apply")
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.WorkingDir, execDir)
	if err != nil {
		return "", "", fmt.Errorf("error while initializing terraform: %s", err)
	}

	stdout := &StdWriter{[]byte{}, true}
	//stderr := &StdWriter{[]byte{}, true}
	tf.SetStdout(stdout)
	//tf.SetStderr(stderr)
	tf.SetStderr(os.Stderr)

	err = tf.Init(context.Background(), tfexec.Upgrade(false))
	if err != nil {
		println("terraform init failed.")
		return stdout.GetString(), "", fmt.Errorf("terraform init failed. %s", err)
	}

	err = tf.WorkspaceNew(context.Background(), terraform.Workspace)

	if err != nil {
		log.Printf("terraform workspace new failed. workspace: %v . dir: %v", terraform.Workspace, terraform.WorkingDir)
		return stdout.GetString(), "", fmt.Errorf("terraform select failed. %s", err)
	}

	err = tf.Apply(context.Background())
	if err != nil {
		println("terraform plan failed.")
		return stdout.GetString(), "", fmt.Errorf("terraform plan failed. %s", err)
	}

	return stdout.GetString(), "", nil
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

func (terraform Terraform) Plan() (bool, string, string, error) {
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
	err = tf.WorkspaceNew(context.Background(), terraform.Workspace)

	if err != nil {
		log.Printf("terraform workspace new failed. workspace: %v . dir: %v", terraform.Workspace, terraform.WorkingDir)
		return false, stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform select failed. %s", err)
	}
	isNonEmptyPlan, err := tf.Plan(context.Background())
	if err != nil {
		println("terraform plan failed. dir: " + terraform.WorkingDir)
		return isNonEmptyPlan, stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform plan failed. %s", err)
	}

	return isNonEmptyPlan, stdout.GetString(), stderr.GetString(), nil
}
