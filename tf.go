package main

type Terraform struct {
	tf TerraformExecutor
}

type TerraformExecutor interface {
	Apply()
	Plan()
}

func (tf *Terraform) Apply() error {
	print("terraform apply")
	return nil
}

func (tf *Terraform) Plan() error {
	print("terraform plan")
	return nil
}
