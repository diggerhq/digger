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

func ExtractRepoName(gitlabURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		return "", err
	}

	// The repository name is typically the last part of the path
	// We use path.Base to handle cases where there might be a trailing slash
	repoName := parsedURL.Path

	// If the URL ends with .git, remove it
	repoName = strings.TrimPrefix(repoName, "/")
	repoName = strings.TrimSuffix(repoName, ".git")

	return repoName, nil
}

func IsInRepoAllowList(repoUrl string) bool {
	allowList := os.Getenv("DIGGER_REPO_ALLOW_LIST")
	allowedReposUrls := strings.Split(allowList, ",")
	// gitlab.com/diggerhq/test
	// https://gitlab.com/diggerhq/test
	allowedRepoNames := lo.Map(allowedReposUrls, func(repoUrl string, i int) string {
		repoName, err := ExtractRepoName(repoUrl)
		if err != nil {
			log.Printf("could not parse repo url %v: %v", repoUrl, err)
		}
		return repoName
	})
	repoName, err := ExtractRepoName(repoUrl)
	if err != nil {
		log.Printf("warning could not parse url: %v", repoUrl)
	}

	exists := lo.Contains(allowedRepoNames, repoName)

	return exists

}
