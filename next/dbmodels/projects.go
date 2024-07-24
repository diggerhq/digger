package dbmodels

import (
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/next/model"
)

func ToDiggerProject(p *model.Project) digger_config.Project {
	return digger_config.Project{
		Name:               p.Name,
		Dir:                p.TerraformWorkingDir,
		Workspace:          "default",
		Terragrunt:         false,
		OpenTofu:           false,
		Workflow:           "default",
		WorkflowFile:       "digger_workflow.yml",
		IncludePatterns:    []string{},
		ExcludePatterns:    []string{},
		DependencyProjects: []string{},
		DriftDetection:     false,
		AwsRoleToAssume:    nil,
		Generated:          false,
	}
}
