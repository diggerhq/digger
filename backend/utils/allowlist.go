package utils

import (
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/samber/lo"
)

func ExtractCleanRepoName(gitlabURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		slog.Error("Failed to parse URL", "url", gitlabURL, "error", err)
		return "", err
	}

	// The repository name is typically the last part of the path
	// We use path.Base to handle cases where there might be a trailing slash
	repoName := parsedURL.Hostname() + parsedURL.Path

	// If the URL ends with .git, remove it
	repoName = strings.TrimSuffix(repoName, ".git")

	slog.Debug("Extracted clean repo name", "originalUrl", gitlabURL, "cleanName", repoName)
	return repoName, nil
}

func IsInRepoAllowList(repoUrl string) bool {
	allowList := os.Getenv("DIGGER_REPO_ALLOW_LIST")
	if allowList == "" {
		slog.Debug("No repo allow list defined, allowing all repos")
		return true
	}

	allowedReposUrls := strings.Split(allowList, ",")
	// gitlab.com/diggerhq/test
	// https://gitlab.com/diggerhq/test

	repoName, err := ExtractCleanRepoName(repoUrl)
	if err != nil {
		slog.Warn("Could not parse repository URL", "url", repoUrl, "error", err)
		return false
	}

	exists := lo.Contains(allowedReposUrls, repoName)
	if exists {
		slog.Debug("Repository is in allow list", "repo", repoName)
	} else {
		slog.Info("Repository is not in allow list", "repo", repoName, "allowList", allowList)
	}

	return exists
}
