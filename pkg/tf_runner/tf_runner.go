package tf_runner

import (
	"digger/pkg/domain"
	"fmt"
)

func Get(runner string) (domain.TerraformRunner, error) {
	if runner == "terragrunt" {
		return &Terragrunt{}, nil
	}

	if runner == "terraform" {
		return &Terraform{}, nil
	}

	return nil, fmt.Errorf("terraform runner '%s' is not valid", runner)
}
