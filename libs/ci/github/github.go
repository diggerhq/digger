package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/scheduler"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"

	"github.com/google/go-github/v61/github"
)

type GithubServiceProvider interface {
	NewService(ghToken, repoName, owner string) (GithubService, error)
}

type GithubServiceProviderBasic struct{}

func (_ GithubServiceProviderBasic) NewService(ghToken, repoName, owner string) (GithubService, error) {
	client := github.NewClient(nil)
	if ghToken != "" {
		client = client.WithAuthToken(ghToken)
	}

	return GithubService{
		Client:   client,
		RepoName: repoName,
		Owner:    owner,
	}, nil
}

type GithubService struct {
	Client   *github.Client
	RepoName string
	Owner    string
}

func (svc GithubService) GetUserTeams(organisation, user string) ([]string, error) {
	teamsResponse, _, err := svc.Client.Teams.ListTeams(context.Background(), organisation, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list github teams: %v", err)
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

func (svc GithubService) GetChangedFiles(prNumber int) ([]string, error) {
	var fileNames []string
	opts := github.ListOptions{PerPage: 100}
	for {
		files, resp, err := svc.Client.PullRequests.ListFiles(context.Background(), svc.Owner, svc.RepoName, prNumber, &opts)
		if err != nil {
			slog.Error("error getting pull request files", "error", err, "prNumber", prNumber)
			return nil, fmt.Errorf("error getting pull request: %v", err)
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

func (svc GithubService) GetChangedFilesForCommit(owner, repo, commitID string) ([]string, error) {
	var fileNames []string
	opts := github.ListOptions{PerPage: 100}

	for {
		commit, resp, err := svc.Client.Repositories.GetCommit(context.Background(), owner, repo, commitID, &opts)
		if err != nil {
			slog.Error("error getting commit files", "error", err, "commitID", commitID)
			return nil, fmt.Errorf("error getting commitfiles: %v", err)
		}
		for _, file := range commit.Files {
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

func (svc GithubService) ListIssues() ([]*ci.Issue, error) {
	allIssues := make([]*ci.Issue, 0)
	opts := &github.IssueListByRepoOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		issues, resp, err := svc.Client.Issues.ListByRepo(context.Background(), svc.Owner, svc.RepoName, opts)
		if err != nil {
			slog.Error("error getting issues", "error", err)
			return nil, fmt.Errorf("error getting pull request files: %v", err)
		}
		for _, issue := range issues {
			if issue.PullRequestLinks != nil {
				// this is an pull request, skip
				continue
			}

			allIssues = append(allIssues, &ci.Issue{ID: int64(*issue.Number), Title: *issue.Title, Body: *issue.Body})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allIssues, nil
}

func (svc GithubService) PublishIssue(title, body string, labels *[]string) (int64, error) {
	githubissue, _, err := svc.Client.Issues.Create(context.Background(), svc.Owner, svc.RepoName, &github.IssueRequest{Title: &title, Body: &body, Labels: labels})
	if err != nil {
		return 0, fmt.Errorf("could not publish issue: %v", err)
	}
	return *githubissue.ID, err
}

func (svc GithubService) UpdateIssue(id int64, title, body string) (int64, error) {
	githubissue, _, err := svc.Client.Issues.Edit(context.Background(), svc.Owner, svc.RepoName, int(id), &github.IssueRequest{Title: &title, Body: &body})
	if err != nil {
		return 0, fmt.Errorf("could not edit issue: %v", err)
	}
	return *githubissue.ID, err
}

func (svc GithubService) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	githubComment, _, err := svc.Client.Issues.CreateComment(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueComment{Body: &comment})
	if err != nil {
		return nil, fmt.Errorf("could not publish comment to PR %v, %v", prNumber, err)
	}
	return &ci.Comment{
		Id:   strconv.FormatInt(*githubComment.ID, 10),
		Body: githubComment.Body,
		Url:  *githubComment.HTMLURL,
	}, err
}

func (svc GithubService) GetComments(prNumber int) ([]ci.Comment, error) {
	comments, _, err := svc.Client.Issues.ListComments(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}})
	commentBodies := make([]ci.Comment, len(comments))
	for i, comment := range comments {
		commentBodies[i] = ci.Comment{
			Id:   strconv.FormatInt(*comment.ID, 10),
			Body: comment.Body,
			Url:  *comment.HTMLURL,
		}
	}
	return commentBodies, err
}

func (svc GithubService) GetApprovals(prNumber int) ([]string, error) {
	reviews, _, err := svc.Client.PullRequests.ListReviews(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.ListOptions{})
	approvals := make([]string, 0)
	for _, review := range reviews {
		if *review.State == "APPROVED" {
			approvals = append(approvals, *review.User.Login)
		}
	}
	return approvals, err
}

func (svc GithubService) EditComment(prNumber int, id, comment string) error {
	commentId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("could not convert id %v to i64: %v", id, err)
	}
	_, _, err = svc.Client.Issues.EditComment(context.Background(), svc.Owner, svc.RepoName, commentId, &github.IssueComment{Body: &comment})
	return err
}

func (svc GithubService) DeleteComment(id string) error {
	commentId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("could not convert id %v to i64: %v", id, err)
	}
	_, err = svc.Client.Issues.DeleteComment(context.Background(), svc.Owner, svc.RepoName, commentId)
	return err
}

type GithubCommentReaction string

const (
	GithubCommentPlusOneReaction  GithubCommentReaction = "+1"
	GithubCommentMinusOneReaction GithubCommentReaction = "-1"
	GithubCommentLaughReaction    GithubCommentReaction = "laugh"
	GithubCommentConfusedReaction GithubCommentReaction = "confused"
	GithubCommentHeartReaction    GithubCommentReaction = "heart"
	GithubCommentHoorayReaction   GithubCommentReaction = "hooray"
	GithubCommentRocketReaction   GithubCommentReaction = "rocket"
	GithubCommentEyesReaction     GithubCommentReaction = "eyes"
)

func (svc GithubService) CreateCommentReaction(id, reaction string) error {
	commentId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("could not convert id %v to i64: %v", id, err)
	}

	_, _, err = svc.Client.Reactions.CreateIssueCommentReaction(context.Background(), svc.Owner, svc.RepoName, commentId, reaction)
	if err != nil {
		slog.Error("could not add reaction to comment", "error", err, "commentId", commentId, "reaction", reaction)
		return fmt.Errorf("could not addd reaction to comment: %v", err)
	}
	return nil
}

func (svc GithubService) IsPullRequest(prNumber int) (bool, error) {
	issue, _, err := svc.Client.Issues.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request (as issue)", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("error getting pull request (as issue): %v", err)
	}
	return issue.IsPullRequest(), nil
}

func (svc GithubService) SetStatus(prNumber int, status, statusContext string) error {
	// we have to check if prNumber is an issue or not
	isPullRequest, err := svc.IsPullRequest(prNumber)
	if err != nil {
		slog.Error("error checking if pull request is issue", "error", err, "prNumber", prNumber)
		return fmt.Errorf("error checking if pull request is issue: %v", err)
	}
	if !isPullRequest {
		slog.Info("issue is not of type pull request, ignoring", "prNumber", prNumber)
		return nil
	}

	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return fmt.Errorf("error getting pull request : %v", err)
	}

	// previously was setting description as "statusContext" but
	// faced some issues with too long strings of > 140 chars:
	// 422 Validation Failed [{Resource:Status Field:description Code:custom Message:description is too long (maximum is 140 characters)}]
	// since description isn't shown in ui setting to blank for now
	description := ""

	_, _, err = svc.Client.Repositories.CreateStatus(context.Background(), svc.Owner, svc.RepoName, *pr.Head.SHA, &github.RepoStatus{
		State:       &status,
		Context:     &statusContext,
		Description: &description,
	})
	return err
}

func (svc GithubService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return "", fmt.Errorf("error getting pull request: %v", err)
	}

	statuses, _, err := svc.Client.Repositories.GetCombinedStatus(context.Background(), svc.Owner, svc.RepoName, pr.Head.GetSHA(), nil)
	if err != nil {
		slog.Error("error getting combined status", "error", err, "prNumber", prNumber, "sha", pr.Head.GetSHA())
		return "", fmt.Errorf("error getting combined status: %v", err)
	}

	return *statuses.State, nil
}

func (svc GithubService) MergePullRequest(prNumber int, mergeStrategy string) error {
	isPullRequest, err := svc.IsPullRequest(prNumber)
	if err != nil {
		slog.Error("error checking if PR is issue", "error", err, "prNumber", prNumber)
		return fmt.Errorf("error checking if PR is issue: %v", err)
	}

	// if it is an issue, close it
	if !isPullRequest {
		closedState := "closed"
		issueRequest := &github.IssueRequest{
			State: &closedState,
		}

		_, _, err := svc.Client.Issues.Edit(context.Background(), svc.Owner, svc.RepoName, prNumber, issueRequest)
		if err != nil {
			slog.Error("error closing issue (merging)", "error", err, "prNumber", prNumber)
			return fmt.Errorf("error closing issue (merging): %v", err)
		}
		return nil
	}

	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return fmt.Errorf("error getting pull request: %v", err)
	}

	_, _, err = svc.Client.PullRequests.Merge(context.Background(), svc.Owner, svc.RepoName, prNumber, "auto-merge", &github.PullRequestOptions{
		MergeMethod: mergeStrategy,
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
		slog.Debug("non-standard mergeable state", "mergeableState", mergeableState)
	}

	return exists
}

func (svc GithubService) IsMergeable(prNumber int) (bool, error) {
	isPullRequest, err := svc.IsPullRequest(prNumber)
	if err != nil {
		slog.Error("could not get pull request type", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("could not get pull request type: %v", err)
	}

	// if this is an issue it will always be merable (closable
	if !isPullRequest {
		return true, nil
	}

	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("error getting pull request: %v", err)
	}
	return pr.GetMergeable() && isMergeableState(pr.GetMergeableState()), nil
}

func (svc GithubService) IsMerged(prNumber int) (bool, error) {
	// we have to check if prNumber is an issue or not
	issue, _, err := svc.Client.Issues.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request (as issue)", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("error getting pull request (as issue): %v", err)
	}

	// if it is an issue, we check if it is "closed" instead of "merged"
	if !issue.IsPullRequest() {
		return issue.GetState() == "closed", nil
	}

	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("error getting pull request: %v", err)
	}
	return *pr.Merged, nil
}

func (svc GithubService) IsClosed(prNumber int) (bool, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("error getting pull request: %v", err)
	}

	return pr.GetState() == "closed", nil
}

func (svc GithubService) SetOutput(prNumber int, key, value string) error {
	gout := os.Getenv("GITHUB_ENV")
	if gout == "" {
		return fmt.Errorf("GITHUB_ENV not set, could not set the output in digger step")
	}
	f, err := os.OpenFile(gout, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("could not open file for writing during digger step")
	}
	_, err = fmt.Fprintf(f, "%v=%v", key, value)
	if err != nil {
		return fmt.Errorf("could not write digger file step")
	}
	f.Close()
	return nil
}

func (svc GithubService) GetBranchName(prNumber int) (string, string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return "", "", fmt.Errorf("error getting pull request: %v", err)
	}

	return pr.Head.GetRef(), pr.Head.GetSHA(), nil
}

func (svc GithubService) GetHeadCommitFromBranch(branch string) (string, string, error) {
	branchInfo, _, err := svc.Client.Repositories.GetBranch(context.Background(), svc.Owner, svc.RepoName, branch, 0)
	if err != nil {
		slog.Error("error fetching branch", "error", err, "branch", branch)
		return "", "", fmt.Errorf("could not retrive branch details: %v", err)
	}

	headCommit := branchInfo.GetCommit()
	sha := headCommit.GetSHA()
	message := headCommit.Commit.GetMessage()

	return sha, message, nil
}

func (svc GithubService) CheckBranchExists(branchName string) (bool, error) {
	_, resp, err := svc.Client.Repositories.GetBranch(context.Background(), svc.Owner, svc.RepoName, branchName, 3)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func ConvertGithubPullRequestEventToJobs(payload *github.PullRequestEvent, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, config digger_config.DiggerConfig, performEnvVarInterpolation bool) ([]scheduler.Job, bool, error) {
	workflows := config.Workflows
	jobs := make([]scheduler.Job, 0)

	defaultBranch := *payload.Repo.DefaultBranch
	prBranch := payload.PullRequest.Head.GetRef()

	coversAllImpactedProjects := true

	for _, project := range impactedProjects {
		workflow, ok := workflows[project.Workflow]
		if !ok {
			slog.Error("failed to find workflow config", "workflow", project.Workflow, "project", project.Name)
			return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
		}

		var skipMerge bool
		if workflow.Configuration != nil {
			skipMerge = workflow.Configuration.SkipMergeCheck
		} else {
			skipMerge = false
		}

		runEnvVars := generic.GetRunEnvVars(defaultBranch, prBranch, project.Name, project.Dir)

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, performEnvVarInterpolation)
		pullRequestNumber := payload.PullRequest.Number

		stateRole, cmdRole := "", ""
		if project.AwsRoleToAssume != nil {
			if project.AwsRoleToAssume.State != "" {
				stateRole = project.AwsRoleToAssume.State
			}

			if project.AwsRoleToAssume.Command != "" {
				cmdRole = project.AwsRoleToAssume.Command
			}
		}

		StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)
		if *payload.Action == "closed" && *payload.PullRequest.Merged && *(payload.PullRequest.Base).Ref == *(payload.Repo).DefaultBranch {
			slog.Info("processing merged PR to default branch",
				"prNumber", *pullRequestNumber,
				"project", project.Name,
				"action", *payload.Action)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           workflow.Configuration.OnCommitToDefault,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          *payload.Repo.FullName,
				RequestedBy:        *payload.Sender.Login,
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		} else if *payload.Action == "opened" || *payload.Action == "reopened" || *payload.Action == "synchronize" {
			slog.Info("processing PR update",
				"prNumber", *pullRequestNumber,
				"project", project.Name,
				"action", *payload.Action)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           workflow.Configuration.OnPullRequestPushed,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          *payload.Repo.FullName,
				RequestedBy:        *payload.Sender.Login,
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		} else if *payload.Action == "closed" {
			slog.Info("processing PR closed",
				"prNumber", *pullRequestNumber,
				"project", project.Name)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           workflow.Configuration.OnPullRequestClosed,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          *payload.Repo.FullName,
				RequestedBy:        *payload.Sender.Login,
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		} else if *payload.Action == "converted_to_draft" {
			var commands []string
			if !config.AllowDraftPRs && len(workflow.Configuration.OnPullRequestConvertedToDraft) == 0 {
				commands = []string{"digger unlock"}
			} else {
				commands = workflow.Configuration.OnPullRequestConvertedToDraft
			}

			slog.Info("processing PR converted to draft",
				"prNumber", *pullRequestNumber,
				"project", project.Name,
				"allowDraftPRs", config.AllowDraftPRs)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           commands,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request_converted_to_draft",
				Namespace:          *payload.Repo.FullName,
				RequestedBy:        *payload.Sender.Login,
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		}
	}
	return jobs, coversAllImpactedProjects, nil
}

func ProcessGitHubEvent(ghEvent interface{}, diggerConfig *digger_config.DiggerConfig, ciService ci.PullRequestService) ([]digger_config.Project, *digger_config.Project, int, error) {
	var impactedProjects []digger_config.Project
	var prNumber int

	switch event := ghEvent.(type) {
	case github.PullRequestEvent:
		prNumber = *event.GetPullRequest().Number
		slog.Info("processing GitHub PR event",
			"prNumber", prNumber,
			"action", *event.Action)

		changedFiles, err := ciService.GetChangedFiles(prNumber)
		if err != nil {
			slog.Error("could not get changed files", "error", err, "prNumber", prNumber)
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects, _ = diggerConfig.GetModifiedProjects(changedFiles)
		slog.Info("identified impacted projects",
			"count", len(impactedProjects),
			"prNumber", prNumber)

	case github.IssueCommentEvent:
		prNumber = *event.GetIssue().Number
		slog.Info("processing GitHub issue comment event",
			"prNumber", prNumber,
			"comment", *event.Comment.Body)

		changedFiles, err := ciService.GetChangedFiles(prNumber)
		if err != nil {
			slog.Error("could not get changed files", "error", err, "prNumber", prNumber)
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects, _ = diggerConfig.GetModifiedProjects(changedFiles)
		requestedProject := scheduler.ParseProjectName(*event.Comment.Body)

		if requestedProject == "" {
			slog.Debug("no specific project requested in comment", "prNumber", prNumber)
			return impactedProjects, nil, prNumber, nil
		}

		slog.Debug("specific project requested in comment",
			"requestedProject", requestedProject,
			"prNumber", prNumber)

		for _, project := range impactedProjects {
			if project.Name == requestedProject {
				slog.Debug("found requested project in impacted projects",
					"project", requestedProject,
					"prNumber", prNumber)
				return impactedProjects, &project, prNumber, nil
			}
		}

		slog.Error("requested project not found in modified projects",
			"requestedProject", requestedProject,
			"prNumber", prNumber)
		return nil, nil, 0, fmt.Errorf("requested project not found in modified projects")

	case github.MergeGroupEvent:
		slog.Debug("merge group event received - not handled")
		return nil, nil, 0, UnhandledMergeGroupEventError

	default:
		slog.Error("unsupported event type", "type", fmt.Sprintf("%T", ghEvent))
		return nil, nil, 0, fmt.Errorf("unsupported event type")
	}
	return impactedProjects, nil, prNumber, nil
}

func ProcessGitHubPullRequestEvent(payload *github.PullRequestEvent, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService ci.PullRequestService) ([]digger_config.Project, map[string]digger_config.ProjectToSourceMapping, int, error) {
	var impactedProjects []digger_config.Project
	prNumber := *payload.PullRequest.Number

	slog.Info("processing GitHub pull request event",
		"prNumber", prNumber,
		"action", *payload.Action)

	changedFiles, err := ciService.GetChangedFiles(prNumber)
	if err != nil {
		slog.Error("could not get changed files", "error", err, "prNumber", prNumber)
		return nil, nil, prNumber, fmt.Errorf("could not get changed files")
	}

	impactedProjects, impactedProjectsSourceLocations := diggerConfig.GetModifiedProjects(changedFiles)
	slog.Info("identified directly impacted projects",
		"count", len(impactedProjects),
		"prNumber", prNumber)

	if diggerConfig.DependencyConfiguration.Mode == digger_config.DependencyConfigurationHard {
		slog.Debug("using hard dependency mode, finding all dependent projects", "prNumber", prNumber)
		originalCount := len(impactedProjects)

		impactedProjects, err = generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
		if err != nil {
			slog.Error("failed to find all projects dependant on impacted projects",
				"error", err,
				"prNumber", prNumber)
			return nil, nil, prNumber, fmt.Errorf("failed to find all projects dependant on impacted projects")
		}

		slog.Info("dependencies resolved",
			"originalCount", originalCount,
			"totalCount", len(impactedProjects),
			"prNumber", prNumber)
	}

	return impactedProjects, impactedProjectsSourceLocations, prNumber, nil
}

func ProcessGitHubPushEvent(payload *github.PushEvent, diggerConfig *digger_config.DiggerConfig, dependencyGraph graph.Graph[string, digger_config.Project], ciService ci.PullRequestService) ([]digger_config.Project, map[string]digger_config.ProjectToSourceMapping, *digger_config.Project, int, error) {
	var impactedProjects []digger_config.Project
	var prNumber int

	commitId := *payload.After
	owner := *payload.Repo.Owner.Login
	repo := *payload.Repo.Name

	slog.Info("processing GitHub push event",
		"commitId", commitId,
		"owner", owner,
		"repo", repo)

	// TODO: Refactor to make generic interface
	changedFiles, err := ciService.(*GithubService).GetChangedFilesForCommit(owner, repo, commitId)
	if err != nil {
		slog.Error("could not get changed files for commit",
			"error", err,
			"commitId", commitId,
			"owner", owner,
			"repo", repo)
		return nil, nil, nil, 0, fmt.Errorf("could not get changed files")
	}

	impactedProjects, impactedProjectsSourceMapping := diggerConfig.GetModifiedProjects(changedFiles)
	slog.Info("identified impacted projects from push",
		"count", len(impactedProjects),
		"commitId", commitId)

	return impactedProjects, impactedProjectsSourceMapping, nil, prNumber, nil
}

func issueCommentEventContainsComment(event interface{}, comment string) bool {
	switch event := event.(type) {
	case github.IssueCommentEvent:
		if strings.Contains(*event.Comment.Body, comment) {
			slog.Debug("comment matches pattern",
				"pattern", comment,
				"commentId", *event.Comment.ID)
			return true
		}
	}
	return false
}

func CheckIfHelpComment(event interface{}) bool {
	result := issueCommentEventContainsComment(event, "digger help")
	if result {
		slog.Debug("help comment detected")
	}
	return result
}

func CheckIfShowProjectsComment(event interface{}) bool {
	result := issueCommentEventContainsComment(event, "digger show-projects")
	if result {
		slog.Debug("show-projects comment detected")
	}
	return result
}
