package main

type TerraformExecutor interface {
	Apply() error
	Plan() error
}

type Terraform struct {
}

func (tf *Terraform) Apply() error {
	print("terraform apply")
	return nil
}

func (tf *Terraform) Plan() error {
	print("terraform plan")
	return nil
}

type MockTerraform struct {
	commands []string
}

func (tf *MockTerraform) Apply() error {
	tf.commands = append(tf.commands, "apply")
	return nil
}

func (tf *MockTerraform) Plan() error {
	tf.commands = append(tf.commands, "plan")
	return nil
}
