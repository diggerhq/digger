package comment_utils

import (
	"fmt"
	"os"
)

func isGitHubActions() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
}

func GetWorkflowUrl() string {
	if isGitHubActions() {
		githubServerURL := os.Getenv("GITHUB_SERVER_URL")  // e.g., https://github.com
		githubRepository := os.Getenv("GITHUB_REPOSITORY") // e.g., diggerhq/demo-opentofu
		githubRunID := os.Getenv("GITHUB_RUN_ID")          // numeric run ID
		workflowURL := fmt.Sprintf("%s/%s/actions/runs/%s", githubServerURL, githubRepository, githubRunID)
		return workflowURL
	} else {
		return "#"
	}
}
