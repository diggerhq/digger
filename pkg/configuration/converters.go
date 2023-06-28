package configuration

import (
	"digger/pkg/core/models"
	"digger/pkg/utils"
	"fmt"
	"github.com/dominikbraun/graph"
	"path/filepath"
)

func copyProjects(projects []*ProjectYaml) []Project {
	result := make([]Project, len(projects))
	for i, p := range projects {
		item := Project{p.Name,
			p.Dir,
			p.Workspace,
			p.Terragrunt,
			p.Workflow,
			p.IncludePatterns,
			p.ExcludePatterns,
			p.DependencyProjects,
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
	result.OnPullRequestClosed = config.OnPullRequestClosed
	result.OnPullRequestPushed = config.OnPullRequestPushed
	result.OnCommitToDefault = config.OnCommitToDefault
	return &result
}

// converts dict of WorkflowYaml's to dict of Workflow's
func copyWorkflows(workflows map[string]*WorkflowYaml) map[string]Workflow {
	result := make(map[string]Workflow, len(workflows))
	for i, w := range workflows {
		if w == nil {
			item := *defaultWorkflow()
			result[i] = item
		} else {
			envVars := copyTerraformEnvConfig(w.EnvVars)
			plan := copyStage(w.Plan)
			apply := copyStage(w.Apply)
			configuration := copyWorkflowConfiguration(w.Configuration)
			item := Workflow{
				envVars,
				plan,
				apply,
				configuration,
			}
			result[i] = item
		}
	}
	return result
}

func ConvertDiggerYamlToConfig(diggerYaml *DiggerConfigYaml, workingDir string, walker DirWalker) (*DiggerConfig, graph.Graph[string, string], error) {
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
		diggerConfig.Workflows = make(map[string]Workflow)
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
			return nil, nil, fmt.Errorf("project name '%s' is duplicated", project.Name)
		}
		projectNames[project.Name] = true
	}

	// check project dependencies exist
	for _, project := range diggerConfig.Projects {
		for _, dependency := range project.DependencyProjects {
			if !projectNames[dependency] {
				return nil, nil, fmt.Errorf("project '%s' depends on '%s' which does not exist", project.Name, dependency)
			}
		}
	}

	if diggerYaml.GenerateProjectsConfig != nil {
		dirs, err := walker.GetDirs(workingDir)
		if err != nil {
			return nil, nil, err
		}

		for _, dir := range dirs {
			includePattern := diggerYaml.GenerateProjectsConfig.Include
			excludePattern := diggerYaml.GenerateProjectsConfig.Exclude
			if utils.MatchIncludeExcludePatternsToFile(dir, []string{includePattern}, []string{excludePattern}) {
				project := Project{Name: filepath.Base(dir), Dir: dir, Workflow: defaultWorkflowName, Workspace: "default"}
				diggerConfig.Projects = append(diggerConfig.Projects, project)
			}
		}
	}

	dependencyGraph, err := CreateProjectDependencyGraph(diggerConfig.Projects)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create project dependency graph: %s", err.Error())
	}

	// if one of the workflows is missing Plan or Apply we copy default values
	for _, w := range diggerConfig.Workflows {
		defaultWorkflow := *defaultWorkflow()
		if w.Plan == nil {
			w.Plan = defaultWorkflow.Plan
		}
		if w.Apply == nil {
			w.Apply = defaultWorkflow.Apply
		}
	}

	return &diggerConfig, dependencyGraph, nil
}

func CreateProjectDependencyGraph(projects []Project) (graph.Graph[string, string], error) {
	g := graph.New(graph.StringHash, graph.Directed(), graph.PreventCycles())
	for _, project := range projects {
		v, _ := g.Vertex(project.Name)

		if v == "" {
			err := g.AddVertex(project.Name)
			if err != nil {
				return nil, err
			}
		}
		for _, dependency := range project.DependencyProjects {
			v, _ := g.Vertex(dependency)

			if v == "" {
				err := g.AddVertex(dependency)

				if err != nil {
					return nil, err
				}
			}

			err := g.AddEdge(dependency, project.Name)
			if err != nil {
				return nil, err
			}
		}
	}
	return g, nil
}
