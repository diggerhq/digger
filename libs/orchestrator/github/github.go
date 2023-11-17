package github

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/diggerhq/digger/libs/digger_config"
	orchestrator "github.com/diggerhq/digger/libs/orchestrator"
	"github.com/dominikbraun/graph"

	"github.com/google/go-github/v55/github"
)

func NewGitHubService(ghToken string, repoName string, owner string) GithubService {
	_, useGithubEnterpiseServer := os.LookupEnv("GITHUB_EE_BASE_URL")
	client := github.NewClient(nil)
	if useGithubEnterpiseServer {
		client, _ = client.WithEnterpriseURLs(
			os.Getenv("GITHUB_EE_BASE_URL"),
			os.Getenv("GITHUB_EE_UPLOAD_URL"),
		)
	}

	if ghToken != "" {
		client = client.WithAuthToken(ghToken)
	}

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
	var fileNames []string
	opts := github.ListOptions{PerPage: 100}
	for {
		files, resp, err := svc.Client.PullRequests.ListFiles(context.Background(), svc.Owner, svc.RepoName, prNumber, &opts)
		if err != nil {
			log.Fatalf("error getting pull request files: %v", err)
		}

		for _, file := range files {
			fileNames = append(fileNames, *file.Filename)
			if file.PreviousFilename != nil {
				fileNames = append(fileNames, *file.PreviousFilename)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return fileNames, nil
}

func (svc *GithubService) PublishComment(prNumber int, comment string) error {
	_, _, err := svc.Client.Issues.CreateComment(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueComment{Body: &comment})
	return err
}

func (svc *GithubService) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	comments, _, err := svc.Client.Issues.ListComments(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}})
	commentBodies := make([]orchestrator.Comment, len(comments))
	for i, comment := range comments {
		commentBodies[i] = orchestrator.Comment{
			Id:   *comment.ID,
			Body: comment.Body,
		}
	}
	return commentBodies, err
}

func (svc *GithubService) GetApprovals(prNumber int) ([]string, error) {
	reviews, _, err := svc.Client.PullRequests.ListReviews(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.ListOptions{})
	approvals := make([]string, 0)
	for _, review := range reviews {
		if *review.State == "APPROVED" {
			approvals = append(approvals, *review.User.Login)
		}
	}
	return approvals, err
}

func (svc *GithubService) EditComment(prNumber int, id interface{}, comment string) error {
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

func (svc *GithubService) GetBranchName(prNumber int) (string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
		return "", err
	}
	return pr.Head.GetRef(), nil
}

func ConvertGithubPullRequestEventToJobs(payload *github.PullRequestEvent, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]orchestrator.Job, bool, error) {
	jobs := make([]orchestrator.Job, 0)

	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
		}

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)
		pullRequestNumber := payload.PullRequest.Number

		if *payload.Action == "closed" && *payload.PullRequest.Merged && *(payload.PullRequest.Base).Ref == *(payload.Repo).DefaultBranch {
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				ProjectWorkflow:   project.Workflow,
				Terragrunt:        project.Terragrunt,
				Commands:          workflow.Configuration.OnCommitToDefault,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:    commandEnvVars,
				StateEnvVars:      stateEnvVars,
				PullRequestNumber: pullRequestNumber,
				EventName:         "pull_request",
				Namespace:         *payload.Repo.FullName,
				RequestedBy:       *payload.Sender.Login,
			})
		} else if *payload.Action == "opened" || *payload.Action == "reopened" || *payload.Action == "synchronize" {
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				ProjectWorkflow:   project.Workflow,
				Terragrunt:        project.Terragrunt,
				OpenTofu:          project.OpenTofu,
				Commands:          workflow.Configuration.OnPullRequestPushed,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:    commandEnvVars,
				StateEnvVars:      stateEnvVars,
				PullRequestNumber: pullRequestNumber,
				EventName:         "pull_request",
				Namespace:         *payload.Repo.FullName,
				RequestedBy:       *payload.Sender.Login,
			})
		} else if *payload.Action == "closed" {
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				ProjectWorkflow:   project.Workflow,
				Terragrunt:        project.Terragrunt,
				OpenTofu:          project.OpenTofu,
				Commands:          workflow.Configuration.OnPullRequestClosed,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:    commandEnvVars,
				StateEnvVars:      stateEnvVars,
				PullRequestNumber: pullRequestNumber,
				EventName:         "pull_request",
				Namespace:         *payload.Repo.FullName,
				RequestedBy:       *payload.Sender.Login,
			})
		}
	}
	return jobs, true, nil
}

func ConvertGithubIssueCommentEventToJobs(payload *github.IssueCommentEvent, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]orchestrator.Job, bool, error) {
	jobs := make([]orchestrator.Job, 0)

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

	diggerCommand := strings.ToLower(*payload.Comment.Body)
	diggerCommand = strings.TrimSpace(diggerCommand)

	for _, command := range supportedCommands {
		if strings.HasPrefix(diggerCommand, command) {
			for _, project := range runForProjects {
				workflow, ok := workflows[project.Workflow]
				if !ok {
					return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
				}
				issueNumber := payload.Issue.Number
				stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

				workspace := project.Workspace
				workspaceOverride, err := orchestrator.ParseWorkspace(*payload.Comment.Body)
				if err != nil {
					return []orchestrator.Job{}, false, err
				}
				if workspaceOverride != "" {
					workspace = workspaceOverride
				}
				jobs = append(jobs, orchestrator.Job{
					ProjectName:       project.Name,
					ProjectDir:        project.Dir,
					ProjectWorkspace:  workspace,
					ProjectWorkflow:   project.Workflow,
					Terragrunt:        project.Terragrunt,
					OpenTofu:          project.OpenTofu,
					Commands:          []string{command},
					ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
					PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
					CommandEnvVars:    commandEnvVars,
					StateEnvVars:      stateEnvVars,
					PullRequestNumber: issueNumber,
					EventName:         "issue_comment",
					Namespace:         *payload.Repo.FullName,
					RequestedBy:       *payload.Sender.Login,
				})
			}
		}
	}
	return jobs, coversAllImpactedProjects, nil
}

func ProcessGitHubEvent(ghEvent interface{}, diggerConfig *digger_config.DiggerConfig, ciService orchestrator.PullRequestService) ([]digger_config.Project, *digger_config.Project, int, error) {
	var impactedProjects []digger_config.Project
	var prNumber int

	switch event := ghEvent.(type) {
	case github.PullRequestEvent:
		prNumber = *event.GetPullRequest().Number
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case github.IssueCommentEvent:
		prNumber = *event.GetIssue().Number
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		requestedProject := orchestrator.ParseProjectName(*event.Comment.Body)

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

func ProcessGitHubPullRequestEvent(payload *github.PullRequestEvent, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService orchestrator.PullRequestService) ([]digger_config.Project, int, error) {
	var impactedProjects []digger_config.Project
	var prNumber int
	prNumber = *payload.PullRequest.Number
	changedFiles, err := ciService.GetChangedFiles(prNumber)

	if err != nil {
		return nil, prNumber, fmt.Errorf("could not get changed files")
	}
	impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)

	if diggerConfig.DependencyConfiguration.Mode == digger_config.DependencyConfigurationHard {
		impactedProjects, err = FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
		if err != nil {
			return nil, prNumber, fmt.Errorf("failed to find all projects dependant on impacted projects")
		}
	}

	return impactedProjects, prNumber, nil
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

func ProcessGitHubIssueCommentEvent(payload *github.IssueCommentEvent, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService orchestrator.PullRequestService) ([]digger_config.Project, *digger_config.Project, int, error) {
	var impactedProjects []digger_config.Project
	var prNumber int

	prNumber = *payload.Issue.Number
	changedFiles, err := ciService.GetChangedFiles(prNumber)

	if err != nil {
		return nil, nil, 0, fmt.Errorf("could not get changed files")
	}

	impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)

	if diggerConfig.DependencyConfiguration.Mode == digger_config.DependencyConfigurationHard {
		impactedProjects, err = FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
		if err != nil {
			return nil, nil, prNumber, fmt.Errorf("failed to find all projects dependant on impacted projects")
		}
	}

	requestedProject := orchestrator.ParseProjectName(*payload.Comment.Body)

	if requestedProject == "" {
		return impactedProjects, nil, prNumber, nil
	}

	for _, project := range impactedProjects {
		if project.Name == requestedProject {
			return impactedProjects, &project, prNumber, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("requested project not found in modified projects")
}

func issueCommentEventContainsComment(event interface{}, comment string) bool {
	switch event.(type) {
	case github.IssueCommentEvent:
		event := event.(github.IssueCommentEvent)
		if strings.Contains(*event.Comment.Body, comment) {
			return true
		}
	}
	return false
}

func CheckIfHelpComment(event interface{}) bool {
	return issueCommentEventContainsComment(event, "digger help")
}

func CheckIfShowProjectsComment(event interface{}) bool {
	return issueCommentEventContainsComment(event, "digger show-projects")
}
