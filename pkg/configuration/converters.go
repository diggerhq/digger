package configuration

import (
	"digger/pkg/core/models"
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/jinzhu/copier"
	"path/filepath"
)

func copyProjects(projects []*ProjectYaml) []ProjectConfig {
	result := make([]ProjectConfig, len(projects))
	for i, p := range projects {
		item := ProjectConfig{p.Name,
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

func copyTerraformEnvConfig(terraformEnvConfig *TerraformEnvConfigYaml) *TerraformEnvConfig {
	if terraformEnvConfig == nil {
		return &TerraformEnvConfig{}
	}
	result := TerraformEnvConfig{}
	result.State = make([]EnvVar, len(terraformEnvConfig.State))
	result.Commands = make([]EnvVar, len(terraformEnvConfig.Commands))

	for i, s := range terraformEnvConfig.State {
		item := EnvVar{
			s.Name,
			s.ValueFrom,
			s.Value,
		}
		result.State[i] = item
	}
	for i, s := range terraformEnvConfig.Commands {
		item := EnvVar{
			s.Name,
			s.ValueFrom,
			s.Value,
		}
		result.Commands[i] = item
	}

	return &result
}

func copyStage(stage *StageYaml) *models.Stage {
	result := models.Stage{}
	result.Steps = make([]models.Step, len(stage.Steps))

	for i, s := range stage.Steps {
		item := models.Step{
			Action:    s.Action,
			Value:     s.Value,
			ExtraArgs: s.ExtraArgs,
			Shell:     s.Shell,
		}
		result.Steps[i] = item
	}
	return &result
}

func copyWorkflowConfiguration(config *WorkflowConfigurationYaml) *WorkflowConfiguration {
	result := WorkflowConfiguration{}
	result.OnPullRequestClosed = make([]string, len(config.OnPullRequestClosed))
	result.OnPullRequestPushed = make([]string, len(config.OnPullRequestPushed))
	result.OnCommitToDefault = make([]string, len(config.OnCommitToDefault))

	result.OnPullRequestClosed = config.OnPullRequestClosed
	result.OnPullRequestPushed = config.OnPullRequestPushed
	result.OnCommitToDefault = config.OnCommitToDefault
	return &result
}

func copyWorkflows(workflows map[string]*WorkflowYaml) map[string]WorkflowConfig {
	result := make(map[string]WorkflowConfig, len(workflows))
	for i, w := range workflows {
		envVars := copyTerraformEnvConfig(w.EnvVars)
		plan := copyStage(w.Plan)
		apply := copyStage(w.Apply)
		configuration := copyWorkflowConfiguration(w.Configuration)
		item := WorkflowConfig{
			envVars,
			plan,
			apply,
			configuration,
		}
		copier.Copy(&w, &item)
		result[i] = item
	}
	return result
}

func ConvertDiggerYamlToConfig(diggerYaml *DiggerConfigYaml, workingDir string, walker DirWalker) (*DiggerConfig, error) {
	var diggerConfig DiggerConfig
	const defaultWorkflowName = "default"

	if diggerYaml.AutoMerge != nil {
		diggerConfig.AutoMerge = *diggerYaml.AutoMerge
	} else {
		diggerConfig.AutoMerge = false
	}

	if diggerYaml.CollectUsageData != nil {
		diggerConfig.CollectUsageData = *diggerYaml.CollectUsageData
	} else {
		diggerConfig.CollectUsageData = true
	}

	// if workflow block is not specified in yaml we create a default one, and add it to every project
	if diggerYaml.Workflows != nil {
		workflows := copyWorkflows(diggerYaml.Workflows)
		diggerConfig.Workflows = workflows

		// provide default workflow if not specified
		if _, ok := diggerConfig.Workflows[defaultWorkflowName]; !ok {
			workflow := *defaultWorkflow()
			diggerConfig.Workflows[defaultWorkflowName] = workflow
		}
	} else {
		workflow := *defaultWorkflow()
		diggerConfig.Workflows = make(map[string]WorkflowConfig)
		diggerConfig.Workflows[defaultWorkflowName] = workflow
	}

	projects := copyProjects(diggerYaml.Projects)
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
				project := ProjectConfig{Name: filepath.Base(dir), Dir: filepath.Join(workingDir, dir), Workflow: defaultWorkflowName}
				diggerConfig.Projects = append(diggerConfig.Projects, project)
			}
		}
	}
	return &diggerConfig, nil
}
