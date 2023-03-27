package main

import (
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/utils"
	"os"
	"strings"
)

func main() {
	diggerConfig, err := digger.NewDiggerConfig("")
	if err != nil {
		println("Failed to read digger config.")
		os.Exit(1)
	}

	lock, err := utils.GetLock()
	if err != nil {
		println("Failed to create lock.")
		os.Exit(1)
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		println("GITHUB_TOKEN is not defined")
		os.Exit(1)
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		println("GITHUB_CONTEXT is not defined")
		os.Exit(1)
	}

	parsedGhContext, err := models.GetGitHubContext(ghContext)

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	impactedProjects, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	if err != nil {
		println(err)
		os.Exit(1)
	}

	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	if err != nil {
		println(err)
		os.Exit(1)
	}

	err = digger.RunCommandsPerProject(commandsToRunPerProject, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, lock, "")
	if err != nil {
		println(err)
		os.Exit(1)
	}
}
