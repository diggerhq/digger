package digger_config

import (
	"errors"
	"fmt"

	"github.com/dominikbraun/graph"
)

const defaultWorkflowName = "default"

// hard - even if dependency project wasn't changed, it will be executed
// soft - if dependency project wasn't changed, it will be skipped
const (
	DependencyConfigurationHard = "hard"
	DependencyConfigurationSoft = "soft"
)

func copyProjects(projects []*ProjectYaml) []Project {
	result := make([]Project, len(projects))
	for i, p := range projects {
		driftDetection := true

		if p.DriftDetection != nil {
			driftDetection = *p.DriftDetection
		}

		var roleToAssume *AssumeRoleForProject = nil
		if p.AwsRoleToAssume != nil {

			// set a default region to us-east-1 the same default AWS uses
			if p.AwsRoleToAssume.AwsRoleRegion == "" {
				p.AwsRoleToAssume.AwsRoleRegion = "us-east-1"
			}

			if p.AwsRoleToAssume.State == "" {
				p.AwsRoleToAssume.State = p.AwsRoleToAssume.Command
			}
			if p.AwsRoleToAssume.Command == "" {
				p.AwsRoleToAssume.Command = p.AwsRoleToAssume.State
			}

			roleToAssume = &AssumeRoleForProject{
				AwsRoleRegion: p.AwsRoleToAssume.AwsRoleRegion,
				State:         p.AwsRoleToAssume.State,
				Command:       p.AwsRoleToAssume.Command,
			}
		}

		workflowFile := "digger_workflow.yml"
		if p.WorkflowFile != nil {
			workflowFile = *p.WorkflowFile
		}

		item := Project{p.Name,
			p.Dir,
			p.Workspace,
			p.Terragrunt,
			p.OpenTofu,
			p.Workflow,
			workflowFile,
			p.IncludePatterns,
			p.ExcludePatterns,
			p.DependencyProjects,
			driftDetection,
			roleToAssume,
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

func copyStage(stage *StageYaml) *Stage {
	result := Stage{}
	result.Steps = make([]Step, len(stage.Steps))

	for i, s := range stage.Steps {
		item := Step{
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
	result.OnPullRequestConvertedToDraft = config.OnPullRequestConvertedToDraft
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

func ConvertDiggerYamlToConfig(diggerYaml *DiggerConfigYaml) (*DiggerConfig, graph.Graph[string, Project], error) {
	var diggerConfig DiggerConfig

	if diggerYaml.DependencyConfiguration != nil {
		diggerConfig.DependencyConfiguration = DependencyConfiguration{
			Mode: diggerYaml.DependencyConfiguration.Mode,
		}
	} else {
		diggerConfig.DependencyConfiguration = DependencyConfiguration{
			Mode: DependencyConfigurationHard,
		}
	}

	if diggerYaml.AutoMerge != nil {
		diggerConfig.AutoMerge = *diggerYaml.AutoMerge
	} else {
		diggerConfig.AutoMerge = false
	}

	if diggerYaml.ApplyAfterMerge != nil {
		diggerConfig.ApplyAfterMerge = *diggerYaml.ApplyAfterMerge
	} else {
		diggerConfig.ApplyAfterMerge = false
	}

	if diggerYaml.CommentRenderMode != nil {
		diggerConfig.CommentRenderMode = *diggerYaml.CommentRenderMode
	} else {
		diggerConfig.CommentRenderMode = CommentRenderModeBasic
	}

	if diggerYaml.MentionDriftedProjectsInPR != nil {
		diggerConfig.MentionDriftedProjectsInPR = *diggerYaml.MentionDriftedProjectsInPR
	} else {
		diggerConfig.MentionDriftedProjectsInPR = false
	}

	if diggerYaml.PrLocks != nil {
		diggerConfig.PrLocks = *diggerYaml.PrLocks
	} else {
		diggerConfig.PrLocks = true
	}

	if diggerYaml.Telemetry != nil {
		diggerConfig.Telemetry = *diggerYaml.Telemetry
	} else {
		diggerConfig.Telemetry = true
	}

	if diggerYaml.TraverseToNestedProjects != nil {
		diggerConfig.TraverseToNestedProjects = *diggerYaml.TraverseToNestedProjects
	} else {
		diggerConfig.TraverseToNestedProjects = false
	}

	if diggerYaml.AllowDraftPRs != nil {
		diggerConfig.AllowDraftPRs = *diggerYaml.AllowDraftPRs
	} else {
		diggerConfig.AllowDraftPRs = false
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

func CreateProjectDependencyGraph(projects []Project) (graph.Graph[string, Project], error) {
	projectHash := func(p Project) string {
		return p.Name
	}

	projectsMap := make(map[string]Project)
	for _, project := range projects {
		projectsMap[project.Name] = project
	}

	g := graph.New(projectHash, graph.Directed(), graph.PreventCycles())
	for _, project := range projects {
		_, err := g.Vertex(project.Name)

		if errors.Is(err, graph.ErrVertexNotFound) {
			err := g.AddVertex(project)
			if err != nil {
				return nil, err
			}
		}
		for _, dependency := range project.DependencyProjects {
			_, err := g.Vertex(dependency)

			if errors.Is(err, graph.ErrVertexNotFound) {
				dependencyProject, ok := projectsMap[dependency]
				if !ok {
					return nil, fmt.Errorf("project '%s' does not exist", dependency)
				}
				err := g.AddVertex(dependencyProject)

				if err != nil {
					return nil, err
				}
			}

			err = g.AddEdge(dependency, project.Name)
			if err != nil {
				return nil, err
			}
		}
	}
	return g, nil
}
