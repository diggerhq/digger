package forgejo

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/comment_utils"
	"github.com/gobwas/glob"
)

type ForgejoServiceProvider interface {
	NewService(baseUrl string, token string, repoName string, owner string) (ForgejoService, error)
}

type ForgejoServiceProviderBasic struct{}

func (ForgejoServiceProviderBasic) NewService(baseUrl string, token string, repoName string, owner string) (ForgejoService, error) {
	client, err := gitea.NewClient(baseUrl, gitea.SetToken(token))
	if err != nil {
		return ForgejoService{}, fmt.Errorf("failed to create forgejo client for %s: %w", baseUrl, err)
	}
	return ForgejoService{Client: client, RepoName: repoName, Owner: owner}, nil
}

type ForgejoService struct {
	Client   *gitea.Client
	RepoName string
	Owner    string
}

func listAll[T any](fetch func(opts gitea.ListOptions) ([]T, *gitea.Response, error)) ([]T, error) {
	all := make([]T, 0)
	opts := gitea.ListOptions{Page: 1, PageSize: 50}
	for {
		items, resp, err := fetch(opts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if resp == nil || resp.NextPage == 0 {
			return all, nil
		}
		opts.Page = resp.NextPage
	}
}

func (svc ForgejoService) getPullRequest(prNumber int) (*gitea.PullRequest, error) {
	pr, _, err := svc.Client.GetPullRequest(svc.Owner, svc.RepoName, int64(prNumber))
	if err != nil {
		return nil, fmt.Errorf("error getting pull request %d: %w", prNumber, err)
	}
	return pr, nil
}

func (svc ForgejoService) GetChangedFiles(prNumber int) ([]string, error) {
	files, err := listAll(func(opts gitea.ListOptions) ([]*gitea.ChangedFile, *gitea.Response, error) {
		return svc.Client.ListPullRequestFiles(svc.Owner, svc.RepoName, int64(prNumber), gitea.ListPullRequestFilesOptions{ListOptions: opts})
	})
	if err != nil {
		return nil, fmt.Errorf("error getting pull request files: %w", err)
	}
	fileNames := make([]string, 0, len(files))
	for _, file := range files {
		fileNames = append(fileNames, file.Filename)
		if file.PreviousFilename != "" {
			fileNames = append(fileNames, file.PreviousFilename)
		}
	}
	return fileNames, nil
}

func (svc ForgejoService) GetUserTeams(organisation string, user string) ([]string, error) {
	teams, err := listAll(func(opts gitea.ListOptions) ([]*gitea.Team, *gitea.Response, error) {
		return svc.Client.ListOrgTeams(organisation, gitea.ListTeamsOptions{ListOptions: opts})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list forgejo teams: %w", err)
	}
	userTeams := make([]string, 0)
	for _, team := range teams {
		_, resp, err := svc.Client.GetTeamMember(team.ID, user)
		if err != nil {
			if resp == nil || resp.StatusCode != http.StatusNotFound {
				slog.Warn("failed to check team membership", "team", team.Name, "error", err)
			}
			continue
		}
		userTeams = append(userTeams, team.Name)
	}
	return userTeams, nil
}

func toComment(comment *gitea.Comment) ci.Comment {
	body := comment.Body
	return ci.Comment{Id: strconv.FormatInt(comment.ID, 10), Body: &body, Url: comment.HTMLURL}
}

func (svc ForgejoService) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	created, _, err := svc.Client.CreateIssueComment(svc.Owner, svc.RepoName, int64(prNumber), gitea.CreateIssueCommentOption{Body: comment})
	if err != nil {
		return nil, fmt.Errorf("error publishing comment: %w", err)
	}
	result := toComment(created)
	return &result, nil
}

func (svc ForgejoService) GetComments(prNumber int) ([]ci.Comment, error) {
	comments, err := listAll(func(opts gitea.ListOptions) ([]*gitea.Comment, *gitea.Response, error) {
		return svc.Client.ListIssueComments(svc.Owner, svc.RepoName, int64(prNumber), gitea.ListIssueCommentOptions{ListOptions: opts})
	})
	if err != nil {
		return nil, fmt.Errorf("error getting comments: %w", err)
	}
	result := make([]ci.Comment, 0, len(comments))
	for _, comment := range comments {
		result = append(result, toComment(comment))
	}
	return result, nil
}

func parseCommentId(id string) (int64, error) {
	commentId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid comment id %q: %w", id, err)
	}
	return commentId, nil
}

func (svc ForgejoService) EditComment(prNumber int, id string, comment string) error {
	commentId, err := parseCommentId(id)
	if err != nil {
		return err
	}
	_, _, err = svc.Client.EditIssueComment(svc.Owner, svc.RepoName, commentId, gitea.EditIssueCommentOption{Body: comment})
	return err
}

func (svc ForgejoService) DeleteComment(id string) error {
	commentId, err := parseCommentId(id)
	if err != nil {
		return err
	}
	_, err = svc.Client.DeleteIssueComment(svc.Owner, svc.RepoName, commentId)
	return err
}

func (svc ForgejoService) CreateCommentReaction(id string, reaction string) error {
	commentId, err := parseCommentId(id)
	if err != nil {
		return err
	}
	_, _, err = svc.Client.PostIssueCommentReaction(svc.Owner, svc.RepoName, commentId, reaction)
	return err
}

func (svc ForgejoService) GetApprovals(prNumber int) ([]string, error) {
	return svc.approvals(prNumber, false)
}

func (svc ForgejoService) approvals(prNumber int, officialOnly bool) ([]string, error) {
	reviews, err := listAll(func(opts gitea.ListOptions) ([]*gitea.PullReview, *gitea.Response, error) {
		return svc.Client.ListPullReviews(svc.Owner, svc.RepoName, int64(prNumber), gitea.ListPullReviewsOptions{ListOptions: opts})
	})
	if err != nil {
		return nil, fmt.Errorf("error getting reviews: %w", err)
	}
	latestReviewState := make(map[string]gitea.ReviewStateType)
	for _, review := range reviews {
		if review.Reviewer == nil || review.State == gitea.ReviewStateComment || review.State == gitea.ReviewStatePending {
			continue
		}
		if officialOnly && !review.Official {
			continue
		}
		if review.Dismissed {
			delete(latestReviewState, review.Reviewer.UserName)
			continue
		}
		latestReviewState[review.Reviewer.UserName] = review.State
	}
	approvals := make([]string, 0)
	for user, state := range latestReviewState {
		if state == gitea.ReviewStateApproved {
			approvals = append(approvals, user)
		}
	}
	return approvals, nil
}

func (svc ForgejoService) SetStatus(prNumber int, status string, statusContext string) error {
	pr, err := svc.getPullRequest(prNumber)
	if err != nil {
		return err
	}
	_, _, err = svc.Client.CreateStatus(svc.Owner, svc.RepoName, pr.Head.Sha, gitea.CreateStatusOption{
		State:     gitea.StatusState(status),
		Context:   statusContext,
		TargetURL: comment_utils.GetWorkflowUrl(),
	})
	if err != nil {
		return fmt.Errorf("error setting status %s on %s: %w", status, pr.Head.Sha, err)
	}
	return nil
}

func (svc ForgejoService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	pr, err := svc.getPullRequest(prNumber)
	if err != nil {
		return "", err
	}
	status, _, err := svc.Client.GetCombinedStatus(svc.Owner, svc.RepoName, pr.Head.Sha)
	if err != nil {
		return "", fmt.Errorf("error getting combined status for %s: %w", pr.Head.Sha, err)
	}
	return string(status.State), nil
}

func (svc ForgejoService) MergePullRequest(prNumber int, mergeStrategy string) error {
	style := gitea.MergeStyle(mergeStrategy)
	if style == "" {
		style = gitea.MergeStyleMerge
	}
	merged, resp, err := svc.Client.MergePullRequest(svc.Owner, svc.RepoName, int64(prNumber), gitea.MergePullRequestOption{Style: style})
	if err != nil {
		return fmt.Errorf("error merging pull request %d: %w", prNumber, err)
	}
	if !merged {
		return fmt.Errorf("pull request %d was not merged, server responded with %s", prNumber, resp.Status)
	}
	return nil
}

func (svc ForgejoService) IsMergeable(prNumber int) (bool, error) {
	pr, err := svc.getPullRequest(prNumber)
	if err != nil {
		return false, err
	}
	if pr.State != gitea.StateOpen || !pr.Mergeable || pr.Draft {
		return false, nil
	}
	base, _, err := svc.Client.GetRepoBranch(svc.Owner, svc.RepoName, pr.Base.Ref)
	if err != nil {
		return false, fmt.Errorf("error getting base branch %s: %w", pr.Base.Ref, err)
	}
	if !base.Protected {
		return true, nil
	}
	if base.RequiredApprovals > 0 {
		approvals, err := svc.approvals(prNumber, true)
		if err != nil {
			return false, err
		}
		if int64(len(approvals)) < base.RequiredApprovals {
			return false, nil
		}
	}
	if base.EnableStatusCheck {
		return svc.requiredStatusesPassed(pr.Head.Sha, base.StatusCheckContexts)
	}
	return true, nil
}

func (svc ForgejoService) requiredStatusesPassed(sha string, requiredContexts []string) (bool, error) {
	combined, _, err := svc.Client.GetCombinedStatus(svc.Owner, svc.RepoName, sha)
	if err != nil {
		return false, fmt.Errorf("error getting combined status for %s: %w", sha, err)
	}
	for _, pattern := range requiredContexts {
		matcher, err := glob.Compile(pattern)
		if err != nil {
			return false, fmt.Errorf("invalid status check pattern %q: %w", pattern, err)
		}
		matched := false
		for _, status := range combined.Statuses {
			if !matcher.Match(status.Context) || strings.HasPrefix(status.Context, "digger/apply") {
				continue
			}
			matched = true
			if status.State != gitea.StatusSuccess {
				return false, nil
			}
		}
		if !matched && !matcher.Match("digger/apply") {
			return false, nil
		}
	}
	return true, nil
}

func (svc ForgejoService) IsMerged(prNumber int) (bool, error) {
	pr, err := svc.getPullRequest(prNumber)
	if err != nil {
		return false, err
	}
	return pr.HasMerged, nil
}

func (svc ForgejoService) IsClosed(prNumber int) (bool, error) {
	pr, err := svc.getPullRequest(prNumber)
	if err != nil {
		return false, err
	}
	return pr.State == gitea.StateClosed, nil
}

func (svc ForgejoService) IsDivergedFromBranch(sourceBranch string, targetBranch string) (bool, error) {
	ahead, _, err := svc.Client.CompareCommits(svc.Owner, svc.RepoName, targetBranch, sourceBranch)
	if err != nil {
		return false, fmt.Errorf("failed to compare %s...%s: %w", targetBranch, sourceBranch, err)
	}
	behind, _, err := svc.Client.CompareCommits(svc.Owner, svc.RepoName, sourceBranch, targetBranch)
	if err != nil {
		return false, fmt.Errorf("failed to compare %s...%s: %w", sourceBranch, targetBranch, err)
	}
	return ahead.TotalCommits > 0 && behind.TotalCommits > 0, nil
}

func (svc ForgejoService) GetBranchName(prNumber int) (string, string, string, string, error) {
	pr, err := svc.getPullRequest(prNumber)
	if err != nil {
		return "", "", "", "", err
	}
	return pr.Head.Ref, pr.Head.Sha, pr.Base.Ref, pr.Base.Sha, nil
}

func (svc ForgejoService) SetOutput(prNumber int, key string, value string) error {
	return generic.AppendToEnvFile(key, value)
}

func (svc ForgejoService) ListIssues() ([]*ci.Issue, error) {
	return nil, fmt.Errorf("implement me")
}

func (svc ForgejoService) PublishIssue(title string, body string, labels *[]string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}

func (svc ForgejoService) UpdateIssue(ID int64, title string, body string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}
