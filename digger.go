package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/mitchellh/mapstructure"
	"os"
)

func main() {
	diggerConfig, err := NewDiggerConfig()
	if err != nil {
		print("Failed to read digger config.")
		os.Exit(1)
	}
	sess := session.Must(session.NewSession())
	dynamoDb := dynamodb.New(sess)

	ghToken := os.Getenv("GITHUB_TOKEN")

	ghContext := os.Getenv("GITHUB_CONTEXT")

	parsedGhContext, err := getGitHubContext(ghContext)
	if ghContext == "" {
		print("GITHUB_CONTEXT is not defined")
		os.Exit(1)
	}

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	repoOwner := parsedGhContext.RepositoryOwner
	repositoryName := parsedGhContext.Repository

	err = processGitHubContext(parsedGhContext, ghEvent, diggerConfig, repoOwner, repositoryName, eventName, dynamoDb, ghToken)
}

func processGitHubContext(parsedGhContext Github, ghEvent map[string]interface{}, diggerConfig *DiggerConfig, repoOwner string, repositoryName string, eventName string, dynamoDb interface{}, ghToken string) error {
	if parsedGhContext.EventName == "pull_request" {

		var parsedGhEvent PullRequestEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing PullRequestEvent: %v", err)
		}

		if parsedGhEvent.PullRequest.Merged {
			print("PR was merged")
		}
		prStatesToLock := []string{"reopened", "opened", "synchronize"}
		prStatesToUnlock := []string{"closed"}

		if contains(prStatesToLock, parsedGhEvent.Action) {
			processNewPullRequest(diggerConfig, repoOwner, repositoryName, eventName, dynamoDb, parsedGhEvent.Number, ghToken)
		} else if contains(prStatesToUnlock, parsedGhEvent.Action) {
			processClosedPullRequest(diggerConfig, repoOwner, repositoryName, eventName, dynamoDb, parsedGhEvent.Number, ghToken)
		}

	} else if parsedGhContext.EventName == "issue_comment" {
		var parsedGhEvent IssueCommentEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing IssueCommentEvent: %v", err)
		}
		print("Issue PR #" + string(rune(parsedGhEvent.Comment.Issue.Number)) + " was commented on")
		processPullRequestComment(diggerConfig, repoOwner, repositoryName, eventName, dynamoDb, parsedGhEvent.Comment.Issue.Number, ghToken, parsedGhEvent.Comment.Body)
	}
	return nil
}

func getGitHubContext(ghContext string) (Github, error) {
	var parsedGhContext Github
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func processNewPullRequest(diggerConfig *DiggerConfig, repoOwner string, repoName string, eventName string, dynamoDb interface{}, prNumber int, ghToken string) {
	print("Processing new PR")
}

func processClosedPullRequest(diggerConfig *DiggerConfig, repoOwner string, repoName string, eventName string, dynamoDb interface{}, prNumber int, ghToken string) {
	print("Processing closed PR")
}

func processPullRequestComment(diggerConfig *DiggerConfig, repoOwner string, repoName string, eventName string, dynamoDb interface{}, prNumber int, ghToken string, commentBody string) {
	print("Processing PR comment")
}
