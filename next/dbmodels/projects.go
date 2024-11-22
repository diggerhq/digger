package dbmodels

import (
	"strings"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/next/model"
)

func ToDiggerProject(p *model.Project) digger_config.Project {
	return digger_config.Project{
		Name: p.Name,
		Dir:  p.TerraformWorkingDir,
		Workspace: func() string {
			if p.Workspace == "" {
				return "default"
			}
			return p.Workspace
		}(),
		Terragrunt: (p.IacType == "terragrunt"),
		OpenTofu:   (p.IacType == "opentofu"),
		Workflow:   p.Workflow,
		WorkflowFile: func() string {
			if p.WorkflowFile == "" {
				return "digger_workflow.yml"
			}
			return p.WorkflowFile
		}(),
		IncludePatterns:    strings.Split(p.IncludePatterns, ","),
		ExcludePatterns:    strings.Split(p.ExcludePatterns, ","),
		DependencyProjects: []string{},
		DriftDetection:     false,
		AwsRoleToAssume:    nil,
		Generated:          false,
	}
}
