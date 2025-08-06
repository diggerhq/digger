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

type ProcessIssueCommentEventResult struct {
	// this represents the projects that need to be planned/ applied for this comment
	ImpactedProjectsSourceMapping map[string]digger_config.ProjectToSourceMapping
	PRNumber                      int
	// this represents all projects impacted by the PR based on changed files
	AllImpactedProjects []digger_config.Project
}

func ProcessIssueCommentEvent(prNumber int, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService ci.PullRequestService) (*ProcessIssueCommentEventResult, error) {
	var impactedProjects []digger_config.Project
	changedFiles, err := ciService.GetChangedFiles(prNumber)

	if err != nil {
		return &ProcessIssueCommentEventResult{}, fmt.Errorf("could not get changed files")
	}

	impactedProjects, impactedProjectsSourceMapping := diggerConfig.GetModifiedProjects(changedFiles)

	if diggerConfig.DependencyConfiguration.Mode == digger_config.DependencyConfigurationHard {
		impactedProjects, err = FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
		if err != nil {
			return &ProcessIssueCommentEventResult{}, fmt.Errorf("failed to find all projects dependant on impacted projects")
		}
	}

	return &ProcessIssueCommentEventResult{
		impactedProjectsSourceMapping,
		prNumber,
		impactedProjects,
	}, nil

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
				return false
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return impactedProjectsWithDependantProjects, nil
}

func ConvertIssueCommentEventToJobs(repoFullName string, requestedBy string, prNumber int, commentBody string, impactedProjectsForComment []digger_config.Project, allImpactedProjects []digger_config.Project, workflows map[string]digger_config.Workflow, prBranchName string, defaultBranch string) ([]scheduler.Job, bool, error) {
	jobs := make([]scheduler.Job, 0)
	prBranch := prBranchName

	supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

	coversAllImpactedProjects := true

	runForProjects := impactedProjectsForComment

	coversAllImpactedProjects = len(impactedProjectsForComment) == len(allImpactedProjects)

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

	jobs, err := CreateJobsForProjects(runForProjects, commandToRun, "issue_comment", repoFullName, requestedBy, workflows, &prNumber, nil, defaultBranch, prBranch, true)
	if err != nil {
		return nil, false, err
	}

	return jobs, coversAllImpactedProjects, nil

}

func CreateJobsForProjects(projects []digger_config.Project, command string, event string, repoFullName string, requestedBy string, workflows map[string]digger_config.Workflow, issueNumber *int, commitSha *string, defaultBranch string, prBranch string, performEnvVarsInterpolations bool) ([]scheduler.Job, error) {
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
		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, performEnvVarsInterpolations)
		StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)
		workspace := project.Workspace
		jobs = append(jobs, scheduler.Job{
			ProjectName:        project.Name,
			ProjectAlias:       project.Alias,
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
			CommandRoleArn:     cmdRole,
			StateRoleArn:       stateRole,
			CognitoOidcConfig:  project.AwsCognitoOidcConfig,
			SkipMergeCheck:     skipMerge,
		})
	}
	return jobs, nil
}
