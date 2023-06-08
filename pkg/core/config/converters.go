package config

import (
	config_yaml "digger/pkg/config/yaml"
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"path/filepath"
)

func convertStageYaml(s *config_yaml.StageYaml) Stage {
	var steps []Step
	for _, step := range s.Steps {
		steps = append(steps, convertStepYaml(&step))
	}
	return Stage{Steps: steps}
}

func convertStepYaml(s *config_yaml.StepYaml) Step {
	return Step{
		Action:    s.Action,
		Value:     s.Value,
		ExtraArgs: s.ExtraArgs,
		Shell:     s.Shell,
	}
}

func convertProjectsYaml(projects []config_yaml.ProjectYaml) []Project {
	result := make([]Project, len(projects))
	for i, p := range projects {
		item := Project{p.Name,
			p.Dir,
			p.Workspace,
			p.Terragrunt,
			p.Workflow,
			p.IncludePatterns,
			p.ExcludePatterns,
		}
		result[i] = item
	}
	return result
}

func convertToProject(p *config_yaml.ProjectYaml) Project {
	item := Project{p.Name,
		p.Dir,
		p.Workspace,
		p.Terragrunt,
		p.Workflow,
		p.IncludePatterns,
		p.ExcludePatterns,
	}
	return item
}

func convertTerraformEnvConfigYaml(envVars *config_yaml.TerraformEnvConfigYaml) *TerraformEnvConfig {
	result := TerraformEnvConfig{}
	result.State = make([]EnvVar, len(envVars.State))
	result.Commands = make([]EnvVar, len(envVars.Commands))

	for i, s := range envVars.State {
		item := EnvVar{
			s.Name,
			s.ValueFrom,
			s.Value,
		}
		result.State[i] = item
	}
	for i, s := range envVars.Commands {
		item := EnvVar{
			s.Name,
			s.ValueFrom,
			s.Value,
		}
		result.Commands[i] = item
	}

	return &result
}

func convertWorkflowConfigurationYaml(config *config_yaml.WorkflowConfigurationYaml) *WorkflowConfiguration {
	result := WorkflowConfiguration{}
	result.OnPullRequestClosed = make([]string, len(config.OnPullRequestClosed))
	result.OnPullRequestPushed = make([]string, len(config.OnPullRequestPushed))
	result.OnCommitToDefault = make([]string, len(config.OnCommitToDefault))

	result.OnPullRequestClosed = config.OnPullRequestClosed
	result.OnPullRequestPushed = config.OnPullRequestPushed
	result.OnCommitToDefault = config.OnCommitToDefault
	return &result
}

func convertWorkflowsYaml(workflows map[string]config_yaml.WorkflowYaml) map[string]Workflow {
	result := make(map[string]Workflow, len(workflows))
	for i, w := range workflows {
		envVars := convertTerraformEnvConfigYaml(w.EnvVars)
		plan := convertStageYaml(w.Plan)
		apply := convertStageYaml(w.Apply)
		configuration := convertWorkflowConfigurationYaml(w.Configuration)
		item := Workflow{
			envVars,
			&plan,
			&apply,
			configuration,
		}
		result[i] = item
	}
	return result
}

func ConvertDiggerYamlToConfig(diggerYaml *config_yaml.DiggerConfigYaml, workingDir string, walker DirWalker) (*DiggerConfig, error) {
	var diggerConfig DiggerConfig
	const defaultWorkflowName = "default"

	diggerConfig.AutoMerge = diggerYaml.AutoMerge

	// if workflow block is not specified in yaml we create a default one, and add it to every project
	if diggerYaml.Workflows != nil {
		workflows := convertWorkflowsYaml(diggerYaml.Workflows)
		diggerConfig.Workflows = workflows

		// provide default workflow if not specified
		if _, ok := diggerConfig.Workflows[defaultWorkflowName]; !ok {
			workflow := *defaultWorkflow()
			diggerConfig.Workflows[defaultWorkflowName] = workflow
		}
	} else {
		workflow := *defaultWorkflow()
		diggerConfig.Workflows = make(map[string]Workflow)
		diggerConfig.Workflows[defaultWorkflowName] = workflow
	}

	projects := convertProjectsYaml(diggerYaml.Projects)
	diggerConfig.Projects = projects

	// update project's workflow if needed
	for _, project := range diggerConfig.Projects {
		if project.Workflow == "" {
			project.Workflow = defaultWorkflowName
		}
	}

	// check for project name duplicates
	projectNames := make(map[string]bool)
	for _, project := range diggerConfig.Projects {
		if projectNames[project.Name] {
			return nil, fmt.Errorf("project name '%s' is duplicated", project.Name)
		}
		projectNames[project.Name] = true
	}
	if diggerYaml.CollectUsageData != nil {
		diggerConfig.CollectUsageData = *diggerYaml.CollectUsageData
	} else {
		diggerConfig.CollectUsageData = true
	}

	if diggerYaml.GenerateProjectsConfig != nil {
		dirs, err := walker.GetDirs(workingDir)
		if err != nil {
			return nil, err
		}

		for _, dir := range dirs {
			includePattern := diggerYaml.GenerateProjectsConfig.Include
			excludePattern := diggerYaml.GenerateProjectsConfig.Exclude
			includeMatch, err := doublestar.PathMatch(includePattern, dir)
			if err != nil {
				return nil, err
			}

			excludeMatch, err := doublestar.PathMatch(excludePattern, dir)
			if err != nil {
				return nil, err
			}
			if includeMatch && !excludeMatch {
				// generate a new project using default workflow
				project := Project{Name: filepath.Base(dir), Dir: filepath.Join(workingDir, dir), Workflow: defaultWorkflowName}
				diggerConfig.Projects = append(diggerConfig.Projects, project)
			}
		}
	}
	return &diggerConfig, nil
}
