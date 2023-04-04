package main

import (
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/utils"
	"fmt"
	"os"
	"strings"
)

func main() {
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		reportErrorAndExit("", "GITHUB_TOKEN is not defined", 1)
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		reportErrorAndExit("", "GITHUB_CONTEXT is not defined", 2)
	}

	parsedGhContext, err := models.GetGitHubContext(ghContext)
	if err != nil {
		reportErrorAndExit("", fmt.Sprintf("Failed to parse GitHub context. %s", err), 3)
	}
	println("GitHub context parsed successfully")

	diggerConfig, err := digger.NewDiggerConfig("")
	if err != nil {
		reportErrorAndExit(parsedGhContext.RepositoryOwner, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	lock, err := utils.GetLock()
	if err != nil {
		reportErrorAndExit(parsedGhContext.RepositoryOwner, fmt.Sprintf("Failed to create lock provider. %s", err), 5)
	}
	println("Lock provider has been created successfully")

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	impactedProjects, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	if err != nil {
		reportErrorAndExit(repoOwner, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
	}
	println("GitHub event processed successfully")

	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	if err != nil {
		reportErrorAndExit(repoOwner, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
	}
	println("GitHub event converted to commands successfully")

	err = digger.RunCommandsPerProject(commandsToRunPerProject, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, lock, "")
	if err != nil {
		reportErrorAndExit(repoOwner, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}
	println("Commands executed successfully")

	reportErrorAndExit(repoOwner, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(repoOwner, fmt.Sprintf("Panic occurred. %s", r), 9)
		}
	}()

}

func reportErrorAndExit(repoOwner string, message string, exitCode int) {
	fmt.Printf(message)
	err := utils.SendLogRecord(repoOwner, message)
	if err != nil {
		fmt.Printf("Failed to send log record. %s\n", err)
	}
	os.Exit(exitCode)
}
