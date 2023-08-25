package github

import (
	"context"
	"digger/pkg/ci"
	"digger/pkg/configuration"
	"digger/pkg/core/lib-orchestrator/github/models"
	dg_models "digger/pkg/core/models"
	"digger/pkg/utils"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v53/github"
)

func NewGitHubService(ghToken string, repoName string, owner string) GithubService {
	client := github.NewTokenClient(context.Background(), ghToken)
	return GithubService{
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

func (svc *GithubService) GetUserTeams(organisation string, user string) ([]string, error) {
	teamsResponse, _, err := svc.Client.Teams.ListTeams(context.Background(), organisation, nil)
	if err != nil {
		log.Fatalf("Failed to list github teams: %v", err)
	}
	var teams []string
	for _, team := range teamsResponse {
		teamMembers, _, _ := svc.Client.Teams.ListTeamMembersBySlug(context.Background(), organisation, *team.Slug, nil)
		for _, member := range teamMembers {
			if *member.Login == user {
				teams = append(teams, *team.Name)
				break
			}
		}
	}

	return teams, nil
}

func (svc *GithubService) GetChangedFiles(prNumber int) ([]string, error) {
	files, _, err := svc.Client.PullRequests.ListFiles(context.Background(), svc.Owner, svc.RepoName, prNumber, nil)
	if err != nil {
		log.Fatalf("error getting pull request files: %v", err)
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

func (svc *GithubService) GetComments(prNumber int) ([]ci.Comment, error) {
	comments, _, err := svc.Client.Issues.ListComments(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}})
	commentBodies := make([]ci.Comment, len(comments))
	for i, comment := range comments {
		commentBodies[i] = ci.Comment{
			Id:   *comment.ID,
			Body: comment.Body,
		}
	}
	return commentBodies, err
}

func (svc *GithubService) EditComment(id interface{}, comment string) error {
	commentId := id.(int64)
	_, _, err := svc.Client.Issues.EditComment(context.Background(), svc.Owner, svc.RepoName, commentId, &github.IssueComment{Body: &comment})
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
		return false, err
	}

	return pr.GetMergeable() && isMergeableState(pr.GetMergeableState()), nil
}

func (svc *GithubService) IsMerged(prNumber int) (bool, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
		return false, err
	}
	return *pr.Merged, nil
}

func (svc *GithubService) IsClosed(prNumber int) (bool, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
		return false, err
	}

	return pr.GetState() == "closed", nil
}

func GetGitHubContext(ghContext string) (*models.GithubAction, error) {
	parsedGhContext := new(models.GithubAction)
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return &models.GithubAction{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

func ConvertGithubEventToJobs(parsedGhContext models.GithubAction, impactedProjects []configuration.Project, requestedProject *configuration.Project, workflows map[string]configuration.Workflow) ([]dg_models.Job, bool, error) {
	jobs := make([]dg_models.Job, 0)

	switch parsedGhContext.Event.(type) {
	case models.PullRequestActionEvent:
		event := parsedGhContext.Event.(models.PullRequestActionEvent)
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}

			stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)

			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				jobs = append(jobs, dg_models.Job{
					ProjectName:       project.Name,
					ProjectDir:        project.Dir,
					ProjectWorkspace:  project.Workspace,
					Terragrunt:        project.Terragrunt,
					Commands:          workflow.Configuration.OnCommitToDefault,
					ApplyStage:        workflow.Apply,
					PlanStage:         workflow.Plan,
					CommandEnvVars:    commandEnvVars,
					StateEnvVars:      stateEnvVars,
					PullRequestNumber: &event.PullRequest.Number,
					EventName:         "pull_request",
					RequestedBy:       parsedGhContext.Actor,
					Namespace:         parsedGhContext.Repository,
				})
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				jobs = append(jobs, dg_models.Job{
					ProjectName:       project.Name,
					ProjectDir:        project.Dir,
					ProjectWorkspace:  project.Workspace,
					Terragrunt:        project.Terragrunt,
					Commands:          workflow.Configuration.OnPullRequestPushed,
					ApplyStage:        workflow.Apply,
					PlanStage:         workflow.Plan,
					CommandEnvVars:    commandEnvVars,
					StateEnvVars:      stateEnvVars,
					PullRequestNumber: &event.PullRequest.Number,
					EventName:         "pull_request",
					Namespace:         parsedGhContext.Repository,
					RequestedBy:       parsedGhContext.Actor,
				})
			} else if event.Action == "closed" {
				jobs = append(jobs, dg_models.Job{
					ProjectName:       project.Name,
					ProjectDir:        project.Dir,
					ProjectWorkspace:  project.Workspace,
					Terragrunt:        project.Terragrunt,
					Commands:          workflow.Configuration.OnPullRequestClosed,
					ApplyStage:        workflow.Apply,
					PlanStage:         workflow.Plan,
					CommandEnvVars:    commandEnvVars,
					StateEnvVars:      stateEnvVars,
					PullRequestNumber: &event.PullRequest.Number,
					EventName:         "pull_request",
					Namespace:         parsedGhContext.Repository,
					RequestedBy:       parsedGhContext.Actor,
				})
			}
		}
		return jobs, true, nil
	case models.IssueCommentActionEvent:
		event := parsedGhContext.Event.(models.IssueCommentActionEvent)
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

		coversAllImpactedProjects := true

		runForProjects := impactedProjects

		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				coversAllImpactedProjects = false
				runForProjects = []configuration.Project{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		diggerCommand := strings.ToLower(event.Comment.Body)
		diggerCommand = strings.TrimSpace(diggerCommand)

		for _, command := range supportedCommands {
			if strings.HasPrefix(diggerCommand, command) {
				for _, project := range runForProjects {
					workflow, ok := workflows[project.Workflow]
					if !ok {
						return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
					}

					stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)

					workspace := project.Workspace
					workspaceOverride, err := utils.ParseWorkspace(event.Comment.Body)
					if err != nil {
						return []dg_models.Job{}, false, err
					}
					if workspaceOverride != "" {
						workspace = workspaceOverride
					}
					jobs = append(jobs, dg_models.Job{
						ProjectName:       project.Name,
						ProjectDir:        project.Dir,
						ProjectWorkspace:  workspace,
						Terragrunt:        project.Terragrunt,
						Commands:          []string{command},
						ApplyStage:        workflow.Apply,
						PlanStage:         workflow.Plan,
						CommandEnvVars:    commandEnvVars,
						StateEnvVars:      stateEnvVars,
						PullRequestNumber: &event.Issue.Number,
						EventName:         "issue_comment",
						RequestedBy:       parsedGhContext.Actor,
						Namespace:         parsedGhContext.Repository,
					})
				}
			}
		}
		return jobs, coversAllImpactedProjects, nil
	default:
		return []dg_models.Job{}, false, fmt.Errorf("unsupported event type: %T", parsedGhContext.EventName)
	}
}

func ProcessGitHubActionEvent(ghEvent models.Event, diggerConfig *configuration.DiggerConfig, ciService ci.PullRequestService) ([]configuration.Project, *configuration.Project, int, error) {
	var impactedProjects []configuration.Project
	var prNumber int

	switch ghEvent.(type) {
	case models.PullRequestActionEvent:
		prNumber = ghEvent.(models.PullRequestActionEvent).PullRequest.Number
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case models.IssueCommentActionEvent:
		prNumber = ghEvent.(models.IssueCommentActionEvent).Issue.Number
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		requestedProject := utils.ParseProjectName(ghEvent.(models.IssueCommentActionEvent).Comment.Body)

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
	case models.IssueCommentActionEvent:
		event := event.(models.IssueCommentActionEvent)
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
