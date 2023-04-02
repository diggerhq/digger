package tf_runner

import (
	"context"
	"digger/pkg/domain"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/terraform-exec/tfexec"
)

type Terraform struct {
	WorkingDir string
	Workspace  string
}

func (terraform *Terraform) SetWorkingDir(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	terraform.WorkingDir = path
	return nil
}

func (terraform *Terraform) Plan() (*domain.TerraformOutput, error) {
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
		log.Print("terraform init failed.")
		return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform init failed. %s", err)
	}

	currentWorkspace, err := tf.WorkspaceShow(context.Background())
	if err != nil {
		log.Printf("terraform workspace show failed. workspace: %v . dir: %v", terraform.Workspace, terraform.WorkingDir)
		return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform show failed. %s", err)
	}

	if currentWorkspace != terraform.Workspace {
		err = tf.WorkspaceNew(context.Background(), terraform.Workspace)

		if err != nil {
			log.Printf("terraform workspace new failed. workspace: %v . dir: %v", terraform.Workspace, terraform.WorkingDir)
			return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform select failed. %s", err)
		}
	}

	_, err = tf.Plan(context.Background())
	if err != nil {
		log.Printf("terraform plan failed. dir: %s", terraform.WorkingDir)
		return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform plan failed. %s", err)
	}

	return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), nil
}

func (terraform *Terraform) Apply() (*domain.TerraformOutput, error) {
	println("digger apply")
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.WorkingDir, execDir)
	if err != nil {
		return domain.NewTerraformOutput("", ""), fmt.Errorf("error while initializing terraform: %s", err)
	}

	stdout := &StdWriter{[]byte{}, true}
	stderr := &StdWriter{[]byte{}, true}
	tf.SetStdout(stdout)
	tf.SetStderr(stderr)

	err = tf.Init(context.Background(), tfexec.Upgrade(false))
	if err != nil {
		return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform init failed. %s", err)
	}
	currentWorkspace, err := tf.WorkspaceShow(context.Background())

	if err != nil {
		log.Printf("terraform workspace show failed. workspace: %v . dir: %v", terraform.Workspace, terraform.WorkingDir)
		return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform show failed. %s", err)
	}

	if currentWorkspace != terraform.Workspace {
		err = tf.WorkspaceNew(context.Background(), terraform.Workspace)

		if err != nil {
			log.Printf("terraform workspace new failed. workspace: %v . dir: %v", terraform.Workspace, terraform.WorkingDir)
			return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform select failed. %s", err)
		}
	}

	err = tf.Apply(context.Background())
	if err != nil {
		println("terraform plan failed.")
		return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), fmt.Errorf("terraform plan failed. %s", err)
	}

	return domain.NewTerraformOutput(stdout.GetString(), stderr.GetString()), nil
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
