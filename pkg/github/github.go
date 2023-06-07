package github

import (
	"context"
	"digger/pkg/ci"
	"digger/pkg/configuration"
	"digger/pkg/github/models"
	dg_models "digger/pkg/models"
	"digger/pkg/utils"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v51/github"
)

func NewGitHubService(ghToken string, repoName string, owner string) ci.CIService {
	client := github.NewTokenClient(context.Background(), ghToken)
	return &GithubService{
		Client:   client,
		RepoName: repoName,
		Owner:    owner,
	}
}

type GithubService struct {
	Client   *github.Client
	RepoName string
	Owner    string
}

func (svc *GithubService) GetChangedFiles(prNumber int) ([]string, error) {
	files, _, err := svc.Client.PullRequests.ListFiles(context.Background(), svc.Owner, svc.RepoName, prNumber, nil)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	fileNames := make([]string, len(files))

	for i, file := range files {
		fileNames[i] = *file.Filename
	}
	return fileNames, nil
}

func (svc *GithubService) PublishComment(prNumber int, comment string) error {
	_, _, err := svc.Client.Issues.CreateComment(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueComment{Body: &comment})
	return err
}

func (svc *GithubService) SetStatus(prNumber int, status string, statusContext string) error {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	_, _, err = svc.Client.Repositories.CreateStatus(context.Background(), svc.Owner, svc.RepoName, *pr.Head.SHA, &github.RepoStatus{
		State:       &status,
		Context:     &statusContext,
		Description: &statusContext,
	})
	return err
}

func (svc *GithubService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	statuses, _, err := svc.Client.Repositories.GetCombinedStatus(context.Background(), svc.Owner, svc.RepoName, pr.Head.GetSHA(), nil)
	if err != nil {
		log.Fatalf("error getting combined status: %v", err)
	}

	return *statuses.State, nil
}

func (svc *GithubService) MergePullRequest(prNumber int) error {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	_, _, err = svc.Client.PullRequests.Merge(context.Background(), svc.Owner, svc.RepoName, prNumber, "auto-merge", &github.PullRequestOptions{
		MergeMethod: "squash",
		SHA:         pr.Head.GetSHA(),
	})
	return err
}

func isMergeableState(mergeableState string) bool {
	// https://docs.github.com/en/github-ae@latest/graphql/reference/enums#mergestatestatus
	mergeableStates := map[string]int{
		"clean":     0,
		"unstable":  0,
		"has_hooks": 1,
	}
	_, exists := mergeableStates[strings.ToLower(mergeableState)]
	if !exists {
		log.Printf("pr.GetMergeableState() returned: %v", mergeableState)
	}

	return exists
}

func (svc *GithubService) IsMergeable(prNumber int) (bool, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	return pr.GetMergeable() && isMergeableState(pr.GetMergeableState()), nil
}

func (svc *GithubService) IsClosed(prNumber int) (bool, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	return pr.GetState() == "closed", nil
}

func GetGitHubContext(ghContext string) (*models.Github, error) {
	parsedGhContext := new(models.Github)
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return &models.Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

func ConvertGithubEventToCommands(event models.Event, impactedProjects []configuration.ProjectConfig, requestedProject *configuration.ProjectConfig, workflows map[string]configuration.WorkflowConfig) ([]dg_models.ProjectCommand, bool, error) {
	commandsPerProject := make([]dg_models.ProjectCommand, 0)

	switch event.(type) {
	case models.PullRequestEvent:
		event := event.(models.PullRequestEvent)
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}

			stateEnvVars, commandEnvVars := configuration.CollectEnvVars(workflow.EnvVars)

			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				commandsPerProject = append(commandsPerProject, dg_models.ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnCommitToDefault,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
				})
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				commandsPerProject = append(commandsPerProject, dg_models.ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnPullRequestPushed,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
				})
			} else if event.Action == "closed" {
				commandsPerProject = append(commandsPerProject, dg_models.ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnPullRequestClosed,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
				})
			}
		}
		return commandsPerProject, true, nil
	case models.IssueCommentEvent:
		event := event.(models.IssueCommentEvent)
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

		coversAllImpactedProjects := true

		runForProjects := impactedProjects

		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				coversAllImpactedProjects = false
				runForProjects = []configuration.ProjectConfig{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				return commandsPerProject, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		for _, command := range supportedCommands {
			if strings.Contains(event.Comment.Body, command) {
				for _, project := range runForProjects {
					workflow, ok := workflows[project.Workflow]
					if !ok {
						return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
					}

					stateEnvVars, commandEnvVars := configuration.CollectEnvVars(workflow.EnvVars)

					workspace := project.Workspace
					workspaceOverride, err := utils.ParseWorkspace(event.Comment.Body)
					if err != nil {
						return []dg_models.ProjectCommand{}, false, err
					}
					if workspaceOverride != "" {
						workspace = workspaceOverride
					}
					commandsPerProject = append(commandsPerProject, dg_models.ProjectCommand{
						ProjectName:      project.Name,
						ProjectDir:       project.Dir,
						ProjectWorkspace: workspace,
						Terragrunt:       project.Terragrunt,
						Commands:         []string{command},
						ApplyStage:       workflow.Apply,
						PlanStage:        workflow.Plan,
						CommandEnvVars:   commandEnvVars,
						StateEnvVars:     stateEnvVars,
					})
				}
			}
		}
		return commandsPerProject, coversAllImpactedProjects, nil
	default:
		return []dg_models.ProjectCommand{}, false, fmt.Errorf("unsupported event type: %T", event)
	}
}

func ProcessGitHubEvent(ghEvent models.Event, diggerConfig *configuration.DiggerConfig, ciService ci.CIService) ([]configuration.ProjectConfig, *configuration.ProjectConfig, int, error) {
	var impactedProjects []configuration.ProjectConfig
	var prNumber int

	switch ghEvent.(type) {
	case models.PullRequestEvent:
		prNumber = ghEvent.(models.PullRequestEvent).PullRequest.Number
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case models.IssueCommentEvent:
		prNumber = ghEvent.(models.IssueCommentEvent).Issue.Number
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		requestedProject := utils.ParseProjectName(ghEvent.(models.IssueCommentEvent).Comment.Body)

		if requestedProject == "" {
			return impactedProjects, nil, prNumber, nil
		}

		for _, project := range impactedProjects {
			if project.Name == requestedProject {
				return impactedProjects, &project, prNumber, nil
			}
		}
		return nil, nil, 0, fmt.Errorf("requested project not found in modified projects")

	default:
		return nil, nil, 0, fmt.Errorf("unsupported event type")
	}
	return impactedProjects, nil, prNumber, nil
}

func issueCommentEventContainsComment(event models.Event, comment string) bool {
	switch event.(type) {
	case models.IssueCommentEvent:
		event := event.(models.IssueCommentEvent)
		if strings.Contains(event.Comment.Body, comment) {
			return true
		}
	}
	return false
}

func CheckIfHelpComment(event models.Event) bool {
	return issueCommentEventContainsComment(event, "digger help")
}

func CheckIfApplyComment(event models.Event) bool {
	return issueCommentEventContainsComment(event, "digger apply")
}
