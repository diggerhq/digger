package config

import (
	"digger/pkg/utils"
	"path"
)

type DiggerConfig struct {
	Projects         []Project
	AutoMerge        bool
	CollectUsageData bool
	Workflows        map[string]Workflow
}

type Project struct {
	Name            string
	Dir             string
	Workspace       string
	Terragrunt      bool
	Workflow        string
	IncludePatterns []string
	ExcludePatterns []string
}

type Workflow struct {
	EnvVars       *TerraformEnvConfig
	Plan          *Stage
	Apply         *Stage
	Configuration *WorkflowConfiguration
}

type WorkflowConfiguration struct {
	OnPullRequestPushed []string
	OnPullRequestClosed []string
	OnCommitToDefault   []string
}

type TerraformEnvConfig struct {
	State    []EnvVar
	Commands []EnvVar
}

type EnvVar struct {
	Name      string
	ValueFrom string
	Value     string
}

type Step struct {
	Action    string
	Value     string
	ExtraArgs []string
	Shell     string
}

type Stage struct {
	Steps []Step
}

type ProjectCommand struct {
	ProjectName      string
	ProjectDir       string
	ProjectWorkspace string
	Terragrunt       bool
	Commands         []string
	ApplyStage       *Stage
	PlanStage        *Stage
	StateEnvVars     map[string]string
	CommandEnvVars   map[string]string
}

func defaultProject() Project {
	return Project{
		Name:       "default",
		Dir:        ".",
		Workspace:  "default",
		Terragrunt: false,
		Workflow:   "default",
	}
}

func (c *DiggerConfig) GetProject(projectName string) *Project {
	for _, project := range c.Projects {
		if projectName == project.Name {
			return &project
		}
	}
	return nil
}

func (c *DiggerConfig) GetProjects(projectName string) []Project {
	if projectName == "" {
		return c.Projects
	}
	project := c.GetProject(projectName)
	if project == nil {
		return nil
	}
	return []Project{*project}
}

func (c *DiggerConfig) GetModifiedProjects(changedFiles []string) []Project {
	var result []Project
	for _, project := range c.Projects {
		for _, changedFile := range changedFiles {
			// we append ** to make our directory a globable pattern
			projectDirPattern := path.Join(project.Dir, "**")
			includePatterns := project.IncludePatterns
			excludePatterns := project.ExcludePatterns
			// all our patterns are the globale dir pattern + the include patterns specified by user
			allIncludePatterns := append([]string{projectDirPattern}, includePatterns...)
			if utils.MatchIncludeExcludePatternsToFile(changedFile, allIncludePatterns, excludePatterns) {
				result = append(result, project)
				break
			}
		}
	}
	return result
}

func (c *DiggerConfig) GetDirectory(projectName string) string {
	project := c.GetProject(projectName)
	if project == nil {
		return ""
	}
	return project.Dir
}

func (c *DiggerConfig) GetWorkflow(workflowName string) *Workflow {
	workflows := c.Workflows

	workflow, ok := workflows[workflowName]
	if !ok {
		return nil
	}
	return &workflow

}

func defaultWorkflow() *Workflow {
	return &Workflow{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
	}
}
