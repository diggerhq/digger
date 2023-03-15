package main

type Terraform struct {
}

type TerraformExecutor interface {
	Apply() error
	Plan() error
}

func (tf *Terraform) Apply() error {
	print("terraform apply")
	return nil
}

func (tf *Terraform) Plan() error {
	print("terraform plan")
	return nil
}
