package generic

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/dominikbraun/graph"
	"strings"
)

func GetRunEnvVars(defaultBranch string, prBranch string, projectName string, projectDir string) map[string]string {
	return map[string]string{
		"DEFAULT_BRANCH": defaultBranch,
		"PR_BRANCH":      prBranch,
		"PROJECT_NAME":   projectName,
		"PROJECT_DIR":    projectDir,
	}
}

func ProcessIssueCommentEvent(prNumber int, commentBody string, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService ci.PullRequestService) ([]digger_config.Project, map[string]digger_config.ProjectToSourceMapping, *digger_config.Project, int, error) {
	var impactedProjects []digger_config.Project
	changedFiles, err := ciService.GetChangedFiles(prNumber)

	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("could not get changed files")
	}

	impactedProjects, impactedProjectsSourceMapping := diggerConfig.GetModifiedProjects(changedFiles)

	if diggerConfig.DependencyConfiguration.Mode == digger_config.DependencyConfigurationHard {
		impactedProjects, err = FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
		if err != nil {
			return nil, nil, nil, prNumber, fmt.Errorf("failed to find all projects dependant on impacted projects")
		}
	}

	requestedProject := scheduler.ParseProjectName(commentBody)

	if requestedProject == "" {
		return impactedProjects, impactedProjectsSourceMapping, nil, prNumber, nil
	}

	for _, project := range impactedProjects {
		if project.Name == requestedProject {
			return impactedProjects, impactedProjectsSourceMapping, &project, prNumber, nil
		}
	}
	return nil, nil, nil, 0, fmt.Errorf("requested project not found in modified projects")
}

func FindAllProjectsDependantOnImpactedProjects(impactedProjects []digger_config.Project, dependencyGraph graph.Graph[string, digger_config.Project]) ([]digger_config.Project, error) {
	impactedProjectsMap := make(map[string]digger_config.Project)
	for _, project := range impactedProjects {
		impactedProjectsMap[project.Name] = project
	}
	visited := make(map[string]bool)
	predecessorMap, err := dependencyGraph.PredecessorMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get predecessor map")
	}
	impactedProjectsWithDependantProjects := make([]digger_config.Project, 0)
	for currentNode := range predecessorMap {
		// find all roots of the graph
		if len(predecessorMap[currentNode]) == 0 {
			err := graph.BFS(dependencyGraph, currentNode, func(node string) bool {
				currentProject, err := dependencyGraph.Vertex(node)
				if err != nil {
					return true
				}
				if _, ok := visited[node]; ok {
					return true
				}
				// add a project if it was impacted
				if _, ok := impactedProjectsMap[node]; ok {
					impactedProjectsWithDependantProjects = append(impactedProjectsWithDependantProjects, currentProject)
					visited[node] = true
					return false
				} else {
					// if a project was not impacted, check if it has a parent that was impacted and add it to the map of impacted projects
					for parent := range predecessorMap[node] {
						if _, ok := impactedProjectsMap[parent]; ok {
							impactedProjectsWithDependantProjects = append(impactedProjectsWithDependantProjects, currentProject)
							impactedProjectsMap[node] = currentProject
							visited[node] = true
							return false
						}
					}
				}
				return true
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return impactedProjectsWithDependantProjects, nil
}

func ConvertIssueCommentEventToJobs(repoFullName string, requestedBy string, prNumber int, commentBody string, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow, prBranchName string, defaultBranch string) ([]scheduler.Job, bool, error) {
	jobs := make([]scheduler.Job, 0)
	prBranch := prBranchName

	supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

	coversAllImpactedProjects := true

	runForProjects := impactedProjects

	if requestedProject != nil {
		if len(impactedProjects) > 1 {
			coversAllImpactedProjects = false
			runForProjects = []digger_config.Project{*requestedProject}
		} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
			return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
		}
	}
	diggerCommand := strings.ToLower(commentBody)
	diggerCommand = strings.TrimSpace(diggerCommand)
	var commandToRun string
	isSupportedCommand := false
	for _, command := range supportedCommands {
		if strings.HasPrefix(diggerCommand, command) {
			isSupportedCommand = true
			commandToRun = command
		}
	}
	if !isSupportedCommand {
		return nil, false, fmt.Errorf("command is not supported: %v", diggerCommand)
	}

	jobs, err := CreateJobsForProjects(runForProjects, commandToRun, "issue_comment", repoFullName, requestedBy, workflows, &prNumber, nil, defaultBranch, prBranch)
	if err != nil {
		return nil, false, err
	}

	return jobs, coversAllImpactedProjects, nil

}

func CreateJobsForProjects(projects []digger_config.Project, command string, event string, repoFullName string, requestedBy string, workflows map[string]digger_config.Workflow, issueNumber *int, commitSha *string, defaultBranch string, prBranch string) ([]scheduler.Job, error) {
	jobs := make([]scheduler.Job, 0)

	for _, project := range projects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			return nil, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
		}

		var skipMerge bool
		if workflow.Configuration != nil {
			skipMerge = workflow.Configuration.SkipMergeCheck
		} else {
			skipMerge = false
		}

		stateRole, cmdRole := "", ""
		if project.AwsRoleToAssume != nil {
			if project.AwsRoleToAssume.State != "" {
				stateRole = project.AwsRoleToAssume.State
			}

			if project.AwsRoleToAssume.Command != "" {
				cmdRole = project.AwsRoleToAssume.Command
			}
		}

		runEnvVars := GetRunEnvVars(defaultBranch, prBranch, project.Name, project.Dir)
		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, false)
		StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)
		workspace := project.Workspace
		jobs = append(jobs, scheduler.Job{
			ProjectName:        project.Name,
			ProjectDir:         project.Dir,
			ProjectWorkspace:   workspace,
			ProjectWorkflow:    project.Workflow,
			Terragrunt:         project.Terragrunt,
			OpenTofu:           project.OpenTofu,
			Pulumi:             project.Pulumi,
			Commands:           []string{command},
			ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
			PlanStage:          scheduler.ToConfigStage(workflow.Plan),
			RunEnvVars:         runEnvVars,
			CommandEnvVars:     commandEnvVars,
			StateEnvVars:       stateEnvVars,
			PullRequestNumber:  issueNumber,
			EventName:          event, //"issue_comment",
			Namespace:          repoFullName,
			RequestedBy:        requestedBy,
			StateEnvProvider:   StateEnvProvider,
			CommandEnvProvider: CommandEnvProvider,
			CommandRoleArn:    	cmdRole,
			StateRoleArn:     	stateRole,
			CognitoOidcConfig:  project.AwsCognitoOidcConfig,
			SkipMergeCheck:     skipMerge,
		})
	}
	return jobs, nil
}
