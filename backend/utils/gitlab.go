package utils

import (
	"fmt"
	"github.com/diggerhq/digger/libs/git_utils"
	"log/slog"
	"os"
	"path"

	orchestrator_gitlab "github.com/diggerhq/digger/libs/ci/gitlab"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/xanzy/go-gitlab"
)

type GitlabProvider interface {
	NewClient(token string) (*gitlab.Client, error)
}

type GitlabClientProvider struct{}

func (g GitlabClientProvider) NewClient(token string) (*gitlab.Client, error) {
	baseUrl := os.Getenv("DIGGER_GITLAB_BASE_URL")
	if baseUrl == "" {
		slog.Debug("Creating GitLab client with default base URL")
		client, err := gitlab.NewClient(token)
		return client, err
	} else {
		slog.Debug("Creating GitLab client with custom base URL", "baseUrl", baseUrl)
		client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseUrl))
		return client, err
	}
}

func GetGitlabService(gh GitlabProvider, projectId int, repoName string, repoFullName string, prNumber int, discussionId string) (*orchestrator_gitlab.GitLabService, error) {
	slog.Debug("Getting GitLab service",
		slog.Group("repository",
			slog.String("name", repoName),
			slog.String("fullName", repoFullName),
			slog.Int("projectId", projectId),
		),
		"prNumber", prNumber,
		"discussionId", discussionId,
	)

	token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")

	client, err := gh.NewClient(token)
	if err != nil {
		slog.Error("Failed to create GitLab client", "error", err)
		return nil, fmt.Errorf("could not get gitlab client: %v", err)
	}

	context := orchestrator_gitlab.GitLabContext{
		ProjectName:      repoName,
		ProjectNamespace: repoFullName,
		ProjectId:        &projectId,
		MergeRequestIId:  &prNumber,
		DiscussionID:     discussionId,
	}

	service := orchestrator_gitlab.GitLabService{Client: client, Context: &context}
	slog.Debug("Successfully created GitLab service",
		"projectId", projectId,
		"repoName", repoName,
	)

	return &service, nil
}

func GetDiggerConfigForBranchGitlab(gh GitlabProvider, projectId int, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, prNumber int, discussionId string) (string, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	slog.Info("Getting Digger config for GitLab branch",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
			slog.Int("projectId", projectId),
			slog.String("cloneUrl", cloneUrl),
		),
		"branch", branch,
		"prNumber", prNumber,
		"discussionId", discussionId,
	)

	token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")

	service, err := GetGitlabService(gh, projectId, repoName, repoFullName, prNumber, discussionId)
	if err != nil {
		slog.Error("Failed to get GitLab service",
			"projectId", projectId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, fmt.Errorf("could not get gitlab service: %v", err)
	}

	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	changedFiles, err := service.GetChangedFiles(prNumber)
	if err != nil {
		slog.Error("Failed to get changed files",
			"projectId", projectId,
			"prNumber", prNumber,
			"error", err,
		)
		return "", nil, nil, fmt.Errorf("error getting changed files")
	}

	slog.Debug("Retrieved changed files",
		"projectId", projectId,
		"prNumber", prNumber,
		"changedFilesCount", len(changedFiles),
	)

	err = git_utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", token, "", func(dir string) error {
		diggerYmlPath := path.Join(dir, "digger.yml")
		diggerYmlBytes, err := os.ReadFile(diggerYmlPath)
		if err != nil {
			slog.Error("Failed to read digger.yml file",
				"path", diggerYmlPath,
				"error", err,
			)
			return fmt.Errorf("error reading digger.yml: %w", err)
		}

		diggerYmlStr = string(diggerYmlBytes)
		slog.Debug("Read digger.yml file",
			"repoFullName", repoFullName,
			"configLength", len(diggerYmlStr),
		)

		config, _, dependencyGraph, err = dg_configuration.LoadDiggerConfig(dir, true, changedFiles)
		if err != nil {
			slog.Error("Failed to load Digger config",
				"projectId", projectId,
				"dir", dir,
				"error", err,
			)
			return err
		}
		return nil
	})

	if err != nil {
		slog.Error("Failed to clone and load config",
			"projectId", projectId,
			"branch", branch,
			"error", err,
		)
		return "", nil, nil, fmt.Errorf("error cloning and loading config")
	}

	projectCount := 0
	if config != nil {
		projectCount = len(config.Projects)
	}

	slog.Info("Digger config loaded successfully",
		"projectId", projectId,
		"projectCount", projectCount,
	)

	return diggerYmlStr, config, dependencyGraph, nil
}
