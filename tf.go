package main

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-exec/tfexec"
	"log"
	"os"
)

type TerraformExecutor interface {
	Apply() (string, string, error)
	Plan() (bool, string, string, error)
}

type Terraform struct {
	workingDir string
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

func (terraform *Terraform) Plan() (bool, string, string, error) {
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.workingDir, execDir)
	if err != nil {
		println("failed to initialise terraform")
		return true, "", "", err
	}

	if err != nil {
		log.Fatal("Error while initializing terraform: " + err.Error())
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

	nonEmptyPlan, err := tf.Plan(context.Background())
	if err != nil {
		println("terraform plan failed.")
		return nonEmptyPlan, stdout.GetString(), stderr.GetString(), fmt.Errorf("terraform plan failed. %s", err)
	}

	return nonEmptyPlan, stdout.GetString(), stderr.GetString(), nil
}

func (terraform *Terraform) Apply() (string, string, error) {
	println("digger apply")
	execDir := "terraform"
	tf, err := tfexec.NewTerraform(terraform.workingDir, execDir)

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

	err = tf.Apply(context.Background())
	if err != nil {
		println("terraform plan failed.")
		return stdout.GetString(), "", fmt.Errorf("terraform plan failed. %s", err)
	}

	return stdout.GetString(), "", nil
}
