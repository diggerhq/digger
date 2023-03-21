package main

import (
	"digger/pkg/aws"
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/terraform"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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
		println("GITHUB_CONTEXT is not defined")
		os.Exit(1)
	}

	tf := terraform.Terraform{}

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	err = digger.ProcessGitHubContext(&parsedGhContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, &tf)
	if err != nil {
		print(err)
		os.Exit(1)
	}
}
