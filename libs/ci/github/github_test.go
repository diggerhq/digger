package github

import (
	"github.com/diggerhq/digger/libs/ci/generic"
	"testing"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/google/go-github/v61/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
)

func TestFindAllProjectsDependantOnImpactedProjects(t *testing.T) {

	projects := []digger_config.Project{
		{
			Name: "a",
		},
		{
			Name:               "b",
			DependencyProjects: []string{"k"},
		},
		{
			Name:               "c",
			DependencyProjects: []string{"b", "a", "i"},
		},
		{
			Name:               "d",
			DependencyProjects: []string{"c"},
		},
		{
			Name:               "e",
			DependencyProjects: []string{"i", "c"},
		},
		{
			Name:               "f",
			DependencyProjects: []string{"e"},
		},
		{
			Name:               "g",
			DependencyProjects: []string{"e"},
		},
		{
			Name: "h",
		},
		{
			Name: "i",
		},
		{
			Name: "j",
		},
		{
			Name: "k",
		},
		{
			Name:               "m",
			DependencyProjects: []string{"h"},
		},
	}

	dependencyGraph, err := digger_config.CreateProjectDependencyGraph(projects)

	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	impactedProjects := []digger_config.Project{
		{
			Name: "a",
		},
		{
			Name: "d",
		},
		{
			Name: "f",
		},
		{
			Name: "g",
		},
		{
			Name: "h",
		},
		{
			Name: "i",
		},
		{
			Name: "j",
		},
		{
			Name: "m",
		},
	}

	impactedProjectsWithDependants, err := generic.FindAllProjectsDependantOnImpactedProjects(impactedProjects, dependencyGraph)
	if err != nil {
		return
	}

	assert.Equal(t, 10, len(impactedProjectsWithDependants))

	projectNames := make([]string, 10)
	for _, project := range impactedProjectsWithDependants {
		projectNames = append(projectNames, project.Name)
	}

	assert.Contains(t, projectNames, "a")
	assert.Contains(t, projectNames, "c")
	assert.Contains(t, projectNames, "d")
	assert.Contains(t, projectNames, "e")
	assert.Contains(t, projectNames, "f")
	assert.Contains(t, projectNames, "g")
	assert.Contains(t, projectNames, "h")
	assert.Contains(t, projectNames, "i")
	assert.Contains(t, projectNames, "j")
	assert.Contains(t, projectNames, "m")
	assert.NotContains(t, projectNames, "k")
	assert.NotContains(t, projectNames, "b")
}

func TestFindAllChangedFilesOfPR(t *testing.T) {
	githubPrService, _ := GithubServiceProviderBasic{}.NewService("", "digger", "diggerhq")
	files, _ := githubPrService.GetChangedFiles(98)
	// 45 changed files including 1 renamed file so the previous filename is included
	assert.Equal(t, 46, len(files))
}

func TestGetApprovalsPaginatesBeyondFirstPage(t *testing.T) {
	// Regression test for PRs with >30 reviews: automated tools can post
	// dozens of COMMENTED reviews before any human approves, pushing the
	// real approvals past GitHub's default page size (30). GetApprovals
	// must paginate, and its latest-state-per-user logic must span pages:
	//  - alice: CHANGES_REQUESTED on page 1, APPROVED on page 2 -> approver
	//  - bob:   APPROVED on page 1, CHANGES_REQUESTED on page 2 -> NOT an approver
	//  - carol: APPROVED on page 2 only -> approver
	review := func(user, state string) *github.PullRequestReview {
		return &github.PullRequestReview{
			User:  &github.User{Login: github.String(user)},
			State: github.String(state),
		}
	}

	pageOne := make([]*github.PullRequestReview, 0, 33)
	for i := 0; i < 31; i++ {
		pageOne = append(pageOne, review("review-bot", "COMMENTED"))
	}
	pageOne = append(pageOne, review("alice", "CHANGES_REQUESTED"), review("bob", "APPROVED"))

	pageTwo := []*github.PullRequestReview{
		review("alice", "APPROVED"),
		review("bob", "CHANGES_REQUESTED"),
		review("carol", "APPROVED"),
	}

	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchPages(
			mock.GetReposPullsReviewsByOwnerByRepoByPullNumber,
			pageOne,
			pageTwo,
		),
	)

	svc := GithubService{
		Client:   github.NewClient(mockedHTTPClient),
		Owner:    "diggerhq",
		RepoName: "digger",
	}

	approvals, err := svc.GetApprovals(1)

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"alice", "carol"}, approvals)
}

func TestListIssuesWithNilBody(t *testing.T) {
	issueNum := 123
	issueTitle := "Drift Issue Without Body"

	pageOne := []*github.Issue{
		{
			Number: &issueNum,
			Title:  &issueTitle,
			Body:   nil,
		},
	}

	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetReposIssuesByOwnerByRepo,
			pageOne,
		),
	)

	svc := GithubService{
		Client:   github.NewClient(mockedHTTPClient),
		Owner:    "diggerhq",
		RepoName: "digger",
	}

	issues, err := svc.ListIssues()

	assert.NoError(t, err)
	assert.Equal(t, 1, len(issues))
	assert.Equal(t, int64(123), issues[0].ID)
	assert.Equal(t, "Drift Issue Without Body", issues[0].Title)
	assert.Equal(t, "", issues[0].Body)
}
