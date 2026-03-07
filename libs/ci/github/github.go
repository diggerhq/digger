package github

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/comment_utils"
	"github.com/diggerhq/digger/libs/scheduler"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"

	"github.com/google/go-github/v61/github"
)

type GithubServiceProvider interface {
	NewService(ghToken string, repoName string, owner string) (GithubService, error)
}

type GithubServiceProviderBasic struct{}

func (_ GithubServiceProviderBasic) NewService(ghToken string, repoName string, owner string) (GithubService, error) {
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

func (svc GithubService) GetUserTeams(organisation string, user string) ([]string, error) {
	var teams []string

	// Paginate through all teams
	opts := &github.ListOptions{PerPage: 100}
	for {
		teamsResponse, resp, err := svc.Client.Teams.ListTeams(context.Background(), organisation, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list github teams: %v", err)
		}

		for _, team := range teamsResponse {
			// Paginate through all team members
			memberOpts := &github.TeamListTeamMembersOptions{
				ListOptions: github.ListOptions{PerPage: 100},
			}
		memberLoop:
			for {
				teamMembers, memberResp, err := svc.Client.Teams.ListTeamMembersBySlug(
					context.Background(), organisation, *team.Slug, memberOpts)
				if err != nil {
					// Log error but continue with other teams
					slog.Warn("failed to list team members", "team", *team.Slug, "error", err)
					break
				}

				for _, member := range teamMembers {
					if *member.Login == user {
						teams = append(teams, *team.Name)
						break memberLoop // Found user, move to next team
					}
				}

				if memberResp.NextPage == 0 {
					break
				}
				memberOpts.Page = memberResp.NextPage
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
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

func (svc GithubService) GetChangedFilesForCommit(owner string, repo string, commitID string) ([]string, error) {
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

func (svc GithubService) PublishIssue(title string, body string, labels *[]string) (int64, error) {
	githubissue, _, err := svc.Client.Issues.Create(context.Background(), svc.Owner, svc.RepoName, &github.IssueRequest{Title: &title, Body: &body, Labels: labels})
	if err != nil {
		return 0, fmt.Errorf("could not publish issue: %v", err)
	}
	return *githubissue.ID, err
}

func (svc GithubService) UpdateIssue(ID int64, title string, body string) (int64, error) {
	githubissue, _, err := svc.Client.Issues.Edit(context.Background(), svc.Owner, svc.RepoName, int(ID), &github.IssueRequest{Title: &title, Body: &body})
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
	var allComments []ci.Comment
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := svc.Client.Issues.ListComments(context.Background(), svc.Owner, svc.RepoName, prNumber, opts)
		if err != nil {
			slog.Error("error getting pull request comments", "error", err, "prNumber", prNumber)
			return nil, fmt.Errorf("error getting pull request comments: %v", err)
		}

		for _, comment := range comments {
			// Add nil checks to prevent potential nil pointer dereference
			var commentId string
			if comment.ID != nil {
				commentId = strconv.FormatInt(*comment.ID, 10)
			}

			var commentUrl string
			if comment.HTMLURL != nil {
				commentUrl = *comment.HTMLURL
			}

			var commentBody *string
			if comment.Body != nil {
				commentBody = comment.Body
			}

			allComments = append(allComments, ci.Comment{
				Id:   commentId,
				Body: commentBody,
				Url:  commentUrl,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

func (svc GithubService) GetApprovals(prNumber int) ([]string, error) {
	reviews, _, err := svc.Client.PullRequests.ListReviews(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Track the latest review state per user
	// GitHub returns reviews in chronological order, so later entries are more recent
	// We need to consider the latest review state, not just any APPROVED review
	latestReviewState := make(map[string]string)
	for _, review := range reviews {
		if review.User == nil || review.User.Login == nil || review.State == nil {
			continue
		}
		// Skip COMMENTED reviews as they don't change approval status
		if *review.State == "COMMENTED" {
			continue
		}
		latestReviewState[*review.User.Login] = *review.State
	}

	// Collect users whose latest review state is APPROVED
	approvals := make([]string, 0)
	for user, state := range latestReviewState {
		if state == "APPROVED" {
			approvals = append(approvals, user)
		}
	}
	return approvals, nil
}

func (svc GithubService) EditComment(prNumber int, id string, comment string) error {
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

const GithubCommentPlusOneReaction GithubCommentReaction = "+1"
const GithubCommentMinusOneReaction GithubCommentReaction = "-1"
const GithubCommentLaughReaction GithubCommentReaction = "laugh"
const GithubCommentConfusedReaction GithubCommentReaction = "confused"
const GithubCommentHeartReaction GithubCommentReaction = "heart"
const GithubCommentHoorayReaction GithubCommentReaction = "hooray"
const GithubCommentRocketReaction GithubCommentReaction = "rocket"
const GithubCommentEyesReaction GithubCommentReaction = "eyes"

func (svc GithubService) CreateCommentReaction(id string, reaction string) error {
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

func (svc GithubService) IsPullRequest(PrNumber int) (bool, error) {
	issue, _, err := svc.Client.Issues.Get(context.Background(), svc.Owner, svc.RepoName, PrNumber)
	if err != nil {
		slog.Error("error getting pull request (as issue)", "error", err, "prNumber", PrNumber)
		return false, fmt.Errorf("error getting pull request (as issue): %v", err)
	}
	return issue.IsPullRequest(), nil
}

func (svc GithubService) SetStatus(prNumber int, status string, statusContext string) error {
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
	targetURl := comment_utils.GetWorkflowUrl()

	_, _, err = svc.Client.Repositories.CreateStatus(context.Background(), svc.Owner, svc.RepoName, *pr.Head.SHA, &github.RepoStatus{
		State:       &status,
		Context:     &statusContext,
		Description: &description,
		TargetURL:   &targetURl,
	})
	return err
}

// modern check runs for github (not the commit status)
func (svc GithubService) CreateCheckRun(name string, status string, conclusion string, title string, summary string, text string, headSHA string, actions []*github.CheckRunAction) (*github.CheckRun, error) {
	client := svc.Client
	owner := svc.Owner
	repoName := svc.RepoName
	opts := github.CreateCheckRunOptions{
		Name:    name,
		HeadSHA: headSHA,               // commit SHA to attach the check to
		Status:  github.String(status), // or "queued" / "in_progress"
		Output: &github.CheckRunOutput{
			Title:   github.String(title),
			Summary: github.String(summary),
			Text:    github.String(text),
		},
	}

	if conclusion != "" {
		opts.Conclusion = github.String(conclusion)
	}

	if actions != nil {
		opts.Actions = actions
	}

	ctx := context.Background()
	checkRun, resp, err := client.Checks.CreateCheckRun(ctx, owner, repoName, opts)

	// Log rate limit information
	if resp != nil {
		limit := resp.Header.Get("X-RateLimit-Limit")
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		reset := resp.Header.Get("X-RateLimit-Reset")

		if limit != "" && remaining != "" {
			limitInt, _ := strconv.Atoi(limit)
			remainingInt, _ := strconv.Atoi(remaining)

			// Calculate percentage remaining
			var percentRemaining float64
			if limitInt > 0 {
				percentRemaining = (float64(remainingInt) / float64(limitInt)) * 100
			}

			// Log based on severity
			if remainingInt == 0 {
				slog.Error("GitHub API rate limit EXHAUSTED",
					"operation", "CreateCheckRun",
					"name", name,
					"limit", limit,
					"remaining", remaining,
					"reset", reset,
					"owner", owner,
					"repo", repoName)
			} else if percentRemaining < 20 {
				slog.Warn("GitHub API rate limit getting LOW",
					"operation", "CreateCheckRun",
					"name", name,
					"limit", limit,
					"remaining", remaining,
					"percentRemaining", fmt.Sprintf("%.1f%%", percentRemaining),
					"reset", reset,
					"owner", owner,
					"repo", repoName)
			} else {
				slog.Debug("GitHub API rate limit status",
					"operation", "CreateCheckRun",
					"name", name,
					"limit", limit,
					"remaining", remaining,
					"percentRemaining", fmt.Sprintf("%.1f%%", percentRemaining))
			}
		}
	}

	return checkRun, err
}

type GithubCheckRunUpdateOptions struct {
	Status     *string
	Conclusion *string
	Title      *string
	Summary    *string
	Text       *string
	Actions    []*github.CheckRunAction
}

func (svc GithubService) UpdateCheckRun(checkRunId string, options GithubCheckRunUpdateOptions) (*github.CheckRun, error) {
	status := options.Status
	conclusion := options.Conclusion
	title := options.Title
	summary := options.Summary
	text := options.Text
	actions := options.Actions

	slog.Debug("Updating check run",
		"checkRunId", checkRunId,
		"status", status,
		"conclusion", conclusion,
		"title", title,
		"summary", summary,
		"text", text,
		"actions", actions,
	)
	client := svc.Client
	owner := svc.Owner
	repoName := svc.RepoName

	checkRunIdInt64, err := strconv.ParseInt(checkRunId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not convert id %v to i64: %v", checkRunId, err)
	}

	ctx := context.Background()

	// Fetch existing check run to preserve annotations and other output data
	existingCheckRun, _, err := client.Checks.GetCheckRun(ctx, owner, repoName, checkRunIdInt64)
	if err != nil {
		slog.Warn("Failed to fetch existing check run, proceeding with update anyway",
			"checkRunId", checkRunId,
			"error", err,
		)
		return nil, fmt.Errorf("could not fetch existing check run: %v", err)
	}

	// Merge existing output with new output, preserving annotations and images
	output := &github.CheckRunOutput{}
	if existingCheckRun.Output != nil {
		// Preserve existing annotations if they exist
		if existingCheckRun.Output.Annotations != nil && len(existingCheckRun.Output.Annotations) > 0 {
			output.Annotations = existingCheckRun.Output.Annotations
		}
		// Preserve existing images if they exist
		if existingCheckRun.Output.Images != nil && len(existingCheckRun.Output.Images) > 0 {
			output.Images = existingCheckRun.Output.Images
		}
	}

	newActions := []*github.CheckRunAction{}
	if actions != nil {
		newActions = actions
	}

	// Update with new values (only update if provided and non-empty)
	if title != nil {
		output.Title = github.String(*title)
	} else if existingCheckRun.Output != nil && existingCheckRun.Output.Title != nil {
		output.Title = existingCheckRun.Output.Title
	}

	if summary != nil {
		output.Summary = github.String(*summary)
	} else if existingCheckRun.Output != nil && existingCheckRun.Output.Summary != nil {
		output.Summary = existingCheckRun.Output.Summary
	}

	if text != nil {
		output.Text = github.String(*text)
	} else if existingCheckRun.Output != nil && existingCheckRun.Output.Text != nil {
		output.Text = existingCheckRun.Output.Text
	}

	var newStatus *string = nil
	if status != nil {
		newStatus = status
	} else {
		newStatus = existingCheckRun.Status
	}

	opts := github.UpdateCheckRunOptions{
		Name:    *existingCheckRun.Name,
		Output:  output,
		Actions: newActions,
	}

	if newStatus != nil {
		opts.Status = github.String(*newStatus)
	}

	if conclusion != nil && *conclusion != "" {
		opts.Conclusion = github.String(*conclusion)
	}

	checkRun, resp, err := client.Checks.UpdateCheckRun(ctx, owner, repoName, checkRunIdInt64, opts)

	// Log rate limit information
	if resp != nil {
		limit := resp.Header.Get("X-RateLimit-Limit")
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		reset := resp.Header.Get("X-RateLimit-Reset")

		if limit != "" && remaining != "" {
			limitInt, _ := strconv.Atoi(limit)
			remainingInt, _ := strconv.Atoi(remaining)

			// Calculate percentage remaining
			var percentRemaining float64
			if limitInt > 0 {
				percentRemaining = (float64(remainingInt) / float64(limitInt)) * 100
			}

			// Log based on severity
			if remainingInt == 0 {
				slog.Error("GitHub API rate limit EXHAUSTED",
					"operation", "UpdateCheckRun",
					"checkRunId", checkRunId,
					"limit", limit,
					"remaining", remaining,
					"reset", reset,
					"owner", owner,
					"repo", repoName)
			} else if percentRemaining < 20 {
				slog.Warn("GitHub API rate limit getting LOW",
					"operation", "UpdateCheckRun",
					"checkRunId", checkRunId,
					"limit", limit,
					"remaining", remaining,
					"percentRemaining", fmt.Sprintf("%.1f%%", percentRemaining),
					"reset", reset,
					"owner", owner,
					"repo", repoName)
			} else {
				slog.Debug("GitHub API rate limit status",
					"operation", "UpdateCheckRun",
					"checkRunId", checkRunId,
					"limit", limit,
					"remaining", remaining,
					"percentRemaining", fmt.Sprintf("%.1f%%", percentRemaining))
			}
		}
	}

	if err != nil {
		slog.Error("Failed to update check run",
			"inputCheckRunId", checkRunId,
			"error", err)
		return checkRun, err
	}

	return checkRun, err
}

func (svc GithubService) GetCheckRunsForCommit(commitSha string) ([]*github.CheckRun, error) {
	ctx := context.Background()
	client := svc.Client
	owner := svc.Owner
	repoName := svc.RepoName

	opts := &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	checkRuns, _, err := client.Checks.ListCheckRunsForRef(ctx, owner, repoName, commitSha, opts)
	if err != nil {
		slog.Error("Failed to list check runs for commit",
			"commitSha", commitSha,
			"error", err)
		return nil, err
	}

	return checkRuns.CheckRuns, nil
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

// IsDivergedFromBranch checks whether sourceBranch has diverged from targetBranch.
// Diverged = both ahead and behind.
func (svc GithubService) IsDivergedFromBranch(sourceBranch string, targetBranch string) (bool, error) {
	ctx := context.Background()

	// Compare the commits between the two branches
	comp, _, err := svc.Client.Repositories.CompareCommits(ctx, svc.Owner, svc.RepoName, targetBranch, sourceBranch, nil)
	if err != nil {
		return false, fmt.Errorf("failed to compare %s..%s: %w", targetBranch, sourceBranch, err)
	}

	// Diverged means both sides have unique commits
	if comp.GetAheadBy() > 0 && comp.GetBehindBy() > 0 {
		return true, nil
	}

	return false, nil
}

func (svc GithubService) IsClosed(prNumber int) (bool, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return false, fmt.Errorf("error getting pull request: %v", err)
	}

	return pr.GetState() == "closed", nil
}

func (svc GithubService) SetOutput(prNumber int, key string, value string) error {
	gout := os.Getenv("GITHUB_ENV")
	if gout == "" {
		return fmt.Errorf("GITHUB_ENV not set, could not set the output in digger step")
	}
	f, err := os.OpenFile(gout, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open file for writing during digger step")
	}
	_, err = f.WriteString(fmt.Sprintf("%v=%v", key, value))
	if err != nil {
		return fmt.Errorf("could not write digger file step")
	}
	f.Close()
	return nil
}

func (svc GithubService) GetBranchName(prNumber int) (string, string, string, string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		slog.Error("error getting pull request", "error", err, "prNumber", prNumber)
		return "", "", "", "", fmt.Errorf("error getting pull request: %v", err)
	}

	targetBranch := pr.Base.GetRef()
	targetSha := pr.Base.GetSHA()
	return pr.Head.GetRef(), pr.Head.GetSHA(), targetBranch, targetSha, nil
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
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// getStringValue safely dereferences a string pointer, returning empty string if nil
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getWorkflowCommands safely retrieves workflow commands, returning empty slice if configuration is nil
func getWorkflowCommands(config *digger_config.WorkflowConfiguration, commandType string) []string {
	if config == nil {
		return []string{}
	}

	switch commandType {
	case "OnCommitToDefault":
		return config.OnCommitToDefault
	case "OnPullRequestPushed":
		return config.OnPullRequestPushed
	case "OnPullRequestClosed":
		return config.OnPullRequestClosed
	case "OnPullRequestConvertedToDraft":
		return config.OnPullRequestConvertedToDraft
	default:
		return []string{}
	}
}

func ConvertGithubPullRequestEventToJobs(payload *github.PullRequestEvent, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, config digger_config.DiggerConfig, performEnvVarInterpolation bool) ([]scheduler.Job, bool, error) {
	workflows := config.Workflows
	jobs := make([]scheduler.Job, 0)

	if payload == nil || payload.Repo == nil || payload.PullRequest == nil {
		return nil, false, fmt.Errorf("invalid payload: missing required fields")
	}

	var defaultBranch string
	if payload.Repo.DefaultBranch != nil {
		defaultBranch = *payload.Repo.DefaultBranch
	} else {
		defaultBranch = "main" // fallback default
	}

	var prBranch string
	if payload.PullRequest.Head != nil {
		prBranch = payload.PullRequest.Head.GetRef()
	}

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
		var pullRequestNumber *int
		if payload.PullRequest.Number != nil {
			pullRequestNumber = payload.PullRequest.Number
		} else {
			defaultPRNumber := 0
			pullRequestNumber = &defaultPRNumber
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

		StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)
		action := getStringValue(payload.Action)

		var isMerged bool
		if payload.PullRequest.Merged != nil {
			isMerged = *payload.PullRequest.Merged
		}

		var baseRef string
		if payload.PullRequest.Base != nil && payload.PullRequest.Base.Ref != nil {
			baseRef = *payload.PullRequest.Base.Ref
		}

		if action == "closed" && isMerged && baseRef == defaultBranch {
			slog.Info("processing merged PR to default branch",
				"prNumber", *pullRequestNumber,
				"project", project.Name,
				"action", action)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectAlias:       project.Alias,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Layer:              project.Layer,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           getWorkflowCommands(workflow.Configuration, "OnCommitToDefault"),
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          getStringValue(payload.Repo.FullName),
				RequestedBy:        getStringValue(payload.Sender.Login),
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		} else if action == "opened" || action == "reopened" || action == "synchronize" {
			slog.Info("processing PR update",
				"prNumber", *pullRequestNumber,
				"project", project.Name,
				"action", action)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectAlias:       project.Alias,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Layer:              project.Layer,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           getWorkflowCommands(workflow.Configuration, "OnPullRequestPushed"),
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          getStringValue(payload.Repo.FullName),
				RequestedBy:        getStringValue(payload.Sender.Login),
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		} else if action == "closed" {
			slog.Info("processing PR closed",
				"prNumber", *pullRequestNumber,
				"project", project.Name)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectAlias:       project.Alias,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Layer:              project.Layer,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           getWorkflowCommands(workflow.Configuration, "OnPullRequestClosed"),
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				RunEnvVars:         runEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				PullRequestNumber:  pullRequestNumber,
				EventName:          "pull_request",
				Namespace:          getStringValue(payload.Repo.FullName),
				RequestedBy:        getStringValue(payload.Sender.Login),
				CommandEnvProvider: CommandEnvProvider,
				CommandRoleArn:     cmdRole,
				StateRoleArn:       stateRole,
				StateEnvProvider:   StateEnvProvider,
				CognitoOidcConfig:  project.AwsCognitoOidcConfig,
				SkipMergeCheck:     skipMerge,
			})
		} else if action == "converted_to_draft" {
			var commands []string
			if config.AllowDraftPRs == false && len(getWorkflowCommands(workflow.Configuration, "OnPullRequestConvertedToDraft")) == 0 {
				commands = []string{"digger unlock"}
			} else {
				commands = getWorkflowCommands(workflow.Configuration, "OnPullRequestConvertedToDraft")
			}

			slog.Info("processing PR converted to draft",
				"prNumber", *pullRequestNumber,
				"project", project.Name,
				"allowDraftPRs", config.AllowDraftPRs)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectAlias:       project.Alias,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				ProjectWorkflow:    project.Workflow,
				Layer:              project.Layer,
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
				Namespace:          getStringValue(payload.Repo.FullName),
				RequestedBy:        getStringValue(payload.Sender.Login),
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
	var prNumber int
	prNumber = *payload.PullRequest.Number
	defaultBranch := *payload.Repo.DefaultBranch
	targetBranch := payload.PullRequest.Base.GetRef()

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

	impactedProjects = generic.FilterTargetBranchForImpactedProjects(impactedProjects, defaultBranch, targetBranch)

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
