package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"log"
	"os"
)

func main() {

	diggerConfig := NewDiggerConfig()
	sess := session.Must(session.NewSession())
	dynamoDb := dynamodb.New(sess)

	ghToken := os.Getenv("GITHUB_TOKEN")

	ghContext := os.Getenv("GITHUB_CONTEXT")

	var parsedGhContext Github
	if ghContext == "" {
		log.Fatal("GITHUB_CONTEXT is not defined")
		os.Exit(1)
	}
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		log.Fatal("Error parsing JSON:", err)
		os.Exit(1)
	}

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	repoOwner := parsedGhContext.RepositoryOwner
	repositoryName := parsedGhContext.Repository

	if parsedGhContext.EventName == "pull_request" {
		var parsedGhEvent PullRequestEvent
		err := json.Unmarshal(ghEvent, &parsedGhEvent)
		if err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
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
		err := json.Unmarshal(ghEvent, &parsedGhEvent)
		if err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}
		print("Issue PR #" + string(rune(parsedGhEvent.Comment.Issue.Number)) + " was commented on")
		processPullRequestComment(diggerConfig, repoOwner, repositoryName, eventName, dynamoDb, parsedGhEvent.Comment.Issue.Number, ghToken, parsedGhEvent.Comment.Body)
	}

}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func processNewPullRequest(diggerConfig *DiggerConfig, repoOwner string, repoName string, eventName string, dynamoDb *dynamodb.DynamoDB, prNumber int, ghToken string) {
	print("Processing new PR")
}

func processClosedPullRequest(diggerConfig *DiggerConfig, repoOwner string, repoName string, eventName string, dynamoDb *dynamodb.DynamoDB, prNumber int, ghToken string) {
	print("Processing closed PR")
}

func processPullRequestComment(diggerConfig *DiggerConfig, repoOwner string, repoName string, eventName string, dynamoDb *dynamodb.DynamoDB, prNumber int, ghToken string, commentBody string) {
	print("Processing PR comment")
}
