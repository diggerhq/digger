package utils

import (
	"github.com/samber/lo"
	"log"
	"net/url"
	"os"
	"strings"
)

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func ExtractCleanRepoName(gitlabURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		return "", err
	}

	// The repository name is typically the last part of the path
	// We use path.Base to handle cases where there might be a trailing slash
	repoName := parsedURL.Hostname() + parsedURL.Path

	// If the URL ends with .git, remove it
	repoName = strings.TrimSuffix(repoName, ".git")

	return repoName, nil
}

func IsInRepoAllowList(repoUrl string) bool {
	allowList := os.Getenv("DIGGER_REPO_ALLOW_LIST")
	if allowList == "" {
		return true
	}
	allowedReposUrls := strings.Split(allowList, ",")
	// gitlab.com/diggerhq/test
	// https://gitlab.com/diggerhq/test

	repoName, err := ExtractCleanRepoName(repoUrl)
	if err != nil {
		log.Printf("warning could not parse url: %v", repoUrl)
	}

	exists := lo.Contains(allowedReposUrls, repoName)

	return exists

}
