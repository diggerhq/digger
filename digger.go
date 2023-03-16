package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/mitchellh/mapstructure"
	"os"
	"strings"
)

func main() {
	diggerConfig, err := NewDiggerConfig()
	if err != nil {
		print("Failed to read digger config.")
		os.Exit(1)
	}
	sess := session.Must(session.NewSession())
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := DynamoDbLock{DynamoDb: dynamoDb}

	ghToken := os.Getenv("GITHUB_TOKEN")

	ghContext := os.Getenv("GITHUB_CONTEXT")

	parsedGhContext, err := getGitHubContext(ghContext)
	if ghContext == "" {
		print("GITHUB_CONTEXT is not defined")
		os.Exit(1)
	}

	tf := Terraform{}

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	repoOwner := parsedGhContext.RepositoryOwner
	repositoryName := parsedGhContext.Repository
	githubPrService := NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	err = processGitHubContext(&parsedGhContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, &tf)
	if err != nil {
		print(err)
		os.Exit(1)
	}
}

func processGitHubContext(parsedGhContext *Github, ghEvent map[string]interface{}, diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, tf TerraformExecutor) error {

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
			err := processNewPullRequest(diggerConfig, prManager, eventName, dynamoDbLock, parsedGhEvent.Number)
			if err != nil {
				return err
			}
		} else if contains(prStatesToUnlock, parsedGhEvent.Action) {
			err := processClosedPullRequest(diggerConfig, prManager, eventName, dynamoDbLock, parsedGhEvent.Number)
			if err != nil {
				return err
			}
		}

	} else if parsedGhContext.EventName == "issue_comment" {
		var parsedGhEvent IssueCommentEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing IssueCommentEvent: %v", err)
		}
		print("Issue PR #" + string(rune(parsedGhEvent.Comment.Issue.Number)) + " was commented on")

		err = processPullRequestComment(diggerConfig, prManager, eventName, dynamoDbLock, tf, parsedGhEvent.Comment.Issue.Number, parsedGhEvent.Comment.Body)
		if err != nil {
			return err
		}
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

func processNewPullRequest(diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, prNumber int) error {
	print("Processing new PR")
	return nil
}

func processClosedPullRequest(diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, prNumber int) error {
	print("Processing closed PR")
	return nil
}

func processPullRequestComment(diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, tf TerraformExecutor, prNumber int, commentBody string) error {
	print("Processing PR comment")
	trimmedComment := strings.TrimSpace(commentBody)
	if trimmedComment == "digger plan" {
		err := tf.Plan()
		if err != nil {
			return err
		}

	} else if trimmedComment == "digger apply" {
		err := tf.Apply()
		if err != nil {
			return err
		}

	} else if trimmedComment == "digger unlock" {

	}
	return nil
}
