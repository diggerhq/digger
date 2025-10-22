package tac

import (
	"testing"
)

func TestDetectReadTerragruntConfigDependencies(t *testing.T) {
	tests := []struct {
		name     string
		hclContent string
		expected []string
	}{
		{
			name: "single read_terragrunt_config call",
			hclContent: `
locals {
  project_vars = read_terragrunt_config(find_in_parent_folders("region.hcl"))
  staff_vars   = read_terragrunt_config(find_in_parent_folders("data/staff.hcl"))
}`,
			expected: []string{"region.hcl", "data/staff.hcl"},
		},
		{
			name: "mixed with other locals",
			hclContent: `
locals {
  project_vars = read_terragrunt_config(find_in_parent_folders("region.hcl"))
  environment = "dev"
  staff_vars   = read_terragrunt_config(find_in_parent_folders("data/staff.hcl"))
}`,
			expected: []string{"region.hcl", "data/staff.hcl"},
		},
		{
			name: "no read_terragrunt_config calls",
			hclContent: `
locals {
  environment = "dev"
  region = "us-east-1"
}`,
			expected: []string{},
		},
		{
			name: "with extra_digger_dependencies",
			hclContent: `
locals {
  extra_digger_dependencies = [
    "some-file.hcl"
  ]
  project_vars = read_terragrunt_config(find_in_parent_folders("region.hcl"))
}`,
			expected: []string{"region.hcl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectReadTerragruntConfigDependencies(tt.hclContent)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d dependencies, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Expected dependency %d to be %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}
