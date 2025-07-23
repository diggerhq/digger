package utils

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"

	"github.com/diggerhq/digger/libs/git_utils"

	orchestrator_bitbucket "github.com/diggerhq/digger/libs/ci/bitbucket"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketProvider interface {
	NewClient(token string) (*bitbucket.Client, error)
}

type BitbucketClientProvider struct{}

func (b BitbucketClientProvider) NewClient(token string) (*bitbucket.Client, error) {
	client := bitbucket.NewOAuthbearerToken(token)
	return client, nil
}

func GetBitbucketService(bb BitbucketProvider, token, repoOwner, repoName string, prNumber int) (*orchestrator_bitbucket.BitbucketAPI, error) {
	slog.Debug("Creating Bitbucket service",
		slog.Group("repository",
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"prNumber", prNumber,
	)

	// token := os.Getenv("DIGGER_BITBUCKET_ACCESS_TOKEN")

	// client, err := bb.NewClient(token)
	// if err != nil {
	//	return nil, fmt.Errorf("could not get bitbucket client: %v", err)
	//}
	// context := orchestrator_bitbucket.BitbucketContext{
	//	RepositoryName:     repoName,
	//	RepositoryFullName: repoFullName,
	//	PullRequestID:      &prNumber,
	//}
	service := orchestrator_bitbucket.BitbucketAPI{
		AuthToken:     token,
		RepoWorkspace: repoOwner,
		RepoName:      repoName,
		HttpClient:    http.Client{},
	}
	return &service, nil
}

func GetDiggerConfigForBitbucketBranch(bb BitbucketProvider, token, repoFullName, repoOwner, repoName, cloneUrl, branch string, prNumber int) (string, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	slog.Info("Getting Digger config for Bitbucket branch",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
			slog.String("cloneUrl", cloneUrl),
		),
		"branch", branch,
		"prNumber", prNumber,
	)

	service, err := GetBitbucketService(bb, token, repoOwner, repoName, prNumber)
	if err != nil {
		slog.Error("Could not get Bitbucket service",
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, fmt.Errorf("could not get bitbucket service: %v", err)
	}

	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	changedFiles, err := service.GetChangedFiles(prNumber)
	if err != nil {
		slog.Error("Error getting changed files",
			"repoFullName", repoFullName,
			"prNumber", prNumber,
			"error", err,
		)
		return "", nil, nil, fmt.Errorf("error getting changed files")
	}

	slog.Debug("Retrieved changed files",
		"repoFullName", repoFullName,
		"prNumber", prNumber,
		"changedFilesCount", len(changedFiles),
	)

	err = git_utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", token, "x-token-auth", func(dir string) error {
		diggerYmlPath := path.Join(dir, "digger.yml")
		diggerYmlBytes, err := os.ReadFile(diggerYmlPath)
		if err != nil {
			slog.Error("Error reading digger.yml file",
				"path", diggerYmlPath,
				"error", err,
			)
			return fmt.Errorf("error reading digger.yml: %w", err)
		}

		diggerYmlStr = string(diggerYmlBytes)
		config, _, dependencyGraph, err = dg_configuration.LoadDiggerConfig(dir, true, changedFiles)
		if err != nil {
			slog.Error("Error loading Digger config",
				"repoFullName", repoFullName,
				"dir", dir,
				"error", err,
			)
			return err
		}
		return nil
	})
	if err != nil {
		slog.Error("Error cloning and loading config",
			"repoFullName", repoFullName,
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
		"repoFullName", repoFullName,
		"projectCount", projectCount,
	)

	return diggerYmlStr, config, dependencyGraph, nil
}
