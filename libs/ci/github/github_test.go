package github

import (
	"testing"

	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/google/go-github/v61/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mergeGroupChangedFilesServiceStub struct {
	base  string
	head  string
	files []string
}

func (s *mergeGroupChangedFilesServiceStub) GetChangedFilesBetweenCommits(base, head string) ([]string, error) {
	s.base = base
	s.head = head
	return s.files, nil
}

func TestProcessGitHubMergeGroupEventUsesSyntheticCommitRange(t *testing.T) {
	config := &digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{Name: "network", Dir: "infra/network", Workflow: "default", Branch: digger_config.DefaultBranchName},
			{Name: "database", Dir: "infra/database", Workflow: "default", Branch: digger_config.DefaultBranchName},
		},
	}
	dependencyGraph, err := digger_config.CreateProjectDependencyGraph(config.Projects)
	require.NoError(t, err)

	service := &mergeGroupChangedFilesServiceStub{files: []string{"infra/network/main.tf"}}
	event := &github.MergeGroupEvent{
		Action: github.String("checks_requested"),
		MergeGroup: &github.MergeGroup{
			BaseSHA: github.String("base-sha"),
			HeadSHA: github.String("merge-group-sha"),
			BaseRef: github.String("refs/heads/main"),
		},
		Repo: &github.Repository{DefaultBranch: github.String("main")},
	}

	projects, _, err := ProcessGitHubMergeGroupEvent(event, config, dependencyGraph, service)

	require.NoError(t, err)
	assert.Equal(t, "base-sha", service.base)
	assert.Equal(t, "merge-group-sha", service.head)
	require.Len(t, projects, 1)
	assert.Equal(t, "network", projects[0].Name)
}

func TestConvertGithubMergeGroupEventToJobsPlansAndAppliesExactSyntheticCommit(t *testing.T) {
	event := &github.MergeGroupEvent{
		Action: github.String("checks_requested"),
		MergeGroup: &github.MergeGroup{
			BaseRef: github.String("refs/heads/main"),
			HeadRef: github.String("refs/heads/gh-readonly-queue/main/pr-42-deadbeef"),
			HeadSHA: github.String("merge-group-sha"),
		},
		Repo: &github.Repository{
			DefaultBranch: github.String("main"),
			FullName:      github.String("acme/infrastructure"),
		},
		Sender: &github.User{Login: github.String("github-merge-queue[bot]")},
	}
	projects := []digger_config.Project{{Name: "network", Dir: "infra/network", Workflow: "default"}}
	config := digger_config.DiggerConfig{Workflows: map[string]digger_config.Workflow{"default": {}}}

	planJobs, applyJobs, err := ConvertGithubMergeGroupEventToJobs(event, projects, config, false)

	require.NoError(t, err)
	require.Len(t, planJobs, 1)
	require.Len(t, applyJobs, 1)
	job := planJobs[0]
	assert.Equal(t, []string{"digger plan"}, job.Commands)
	assert.Equal(t, []string{"digger apply"}, applyJobs[0].Commands)
	assert.Equal(t, "merge_group", job.EventName)
	assert.Equal(t, "merge-group-sha", job.PlanIdentifier)
	assert.Nil(t, job.PullRequestNumber)
	assert.True(t, job.SkipMergeCheck)
	assert.True(t, job.SkipProjectLock)
}

func TestChangedFileNamesFromComparisonFailsClosedAtGitHubLimit(t *testing.T) {
	files := make([]*github.CommitFile, githubCompareFileLimit)
	for i := range files {
		files[i] = &github.CommitFile{Filename: github.String("infra/file.tf")}
	}

	_, err := changedFileNamesFromComparison(&github.CommitsComparison{Files: files})

	require.ErrorContains(t, err, "reached GitHub's limit")
}

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
