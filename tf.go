package main

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-exec/tfexec"
	"os"
)

type TerraformExecutor interface {
	Apply() (string, string, error)
	Plan() (string, string, error)
}

type Terraform struct {
	workingDir string
}

func (terraform *Terraform) Apply() (string, string, error) {
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.workingDir, execDir)

	stdout := &StdWriter{[]byte{}, true}
	//stderr := &StdWriter{[]byte{}, true}
	tf.SetStdout(stdout)
	//tf.SetStderr(stderr)
	tf.SetStderr(os.Stderr)

	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		print("terraform init failed.")
		return stdout.GetString(), "", fmt.Errorf("terraform init failed. %s", err)
	}

	err = tf.Apply(context.Background())
	if err != nil {
		print("terraform plan failed.")
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

func (terraform *Terraform) Plan() (string, string, error) {
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.workingDir, execDir)

	stdout := &StdWriter{[]byte{}, true}
	stderr := &StdWriter{[]byte{}, true}
	tf.SetStdout(stdout)
	tf.SetStderr(stderr)

	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		print("terraform init failed.")
		return stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform init failed. %s", err)
	}

	_, err = tf.Plan(context.Background())
	if err != nil {
		print("terraform plan failed.")
		return stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform plan failed. %s", err)
	}

	return stdout.GetString(), stderr.GetString(), nil
}
