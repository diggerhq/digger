package main

import (
	"digger/pkg/aws"
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/models"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"log"
	"os"
	"strings"
)

func main() {
	diggerConfig, err := digger.NewDiggerConfig("")
	if err != nil {
		println("Failed to read digger config.")
		os.Exit(1)
	}
	sess := session.Must(session.NewSession())
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}

	ghToken := os.Getenv("GITHUB_TOKEN")

	ghContext := os.Getenv("GITHUB_CONTEXT")

	parsedGhContext, err := models.GetGitHubContext(ghContext)
	if ghContext == "" {
		log.Fatal("GITHUB_CONTEXT is not defined")
	}

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	impactedProjects, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	if err != nil {
		log.Fatalf("Error processing github event: %v", err)
	}

	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	if err != nil {
		log.Fatalf("Error mapping github event to commands: %v", err)
	}

	err = digger.RunCommandsPerProject(commandsToRunPerProject, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, "")
	if err != nil {
		log.Fatalf("Error running commands per project: %v", err)
	}
}
