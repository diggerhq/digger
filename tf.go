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
