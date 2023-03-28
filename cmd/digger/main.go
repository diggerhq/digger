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
	diggerConfig, err := digger.NewDiggerConfig("")
	if err != nil {
		fmt.Printf("Failed to read digger config. %s\n", err)
		os.Exit(1)
	}
	println("Digger config read successfully")

	lock, err := utils.GetLock()
	if err != nil {
		fmt.Printf("Failed to create lock. %s\n", err)
		os.Exit(2)
	}
	println("Lock has been created successfully")

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		println("GITHUB_TOKEN is not defined")
		os.Exit(3)
	}

	bitbucketContext := os.Getenv("BITBUCKET_CONTEXT")
	if bitbucketContext != "" {
		fmt.Println("BITBUCKET_CONTEXT found")
	}

	githubContext := os.Getenv("GITHUB_CONTEXT")
	if githubContext == "" && bitbucketContext == "" {
		fmt.Printf("GITHUB_CONTEXT is not defined. %s\n", err)
		os.Exit(4)
	}

	if githubContext != "" {
		parsedGhContext, err := models.GetGitHubContext(githubContext)
		if err != nil {
			fmt.Printf("failed to parse GitHub context. %s\n", err.Error())
			os.Exit(5)
		}
		println("GitHub context parsed successfully")

		ghEvent := parsedGhContext.Event
		eventName := parsedGhContext.EventName
		splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")

		repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
		githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

		impactedProjects, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
		if err != nil {
			fmt.Printf("failed to process GitHub event, %v", err)
			os.Exit(6)
		}
		println("GitHub event processed successfully")

		commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
		if err != nil {
			fmt.Printf("failed to convert event to command, %v", err)
			os.Exit(7)
		}
		println("GitHub event converted to commands successfully")

		err = digger.RunCommandsPerProject(commandsToRunPerProject, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, lock, "")
		if err != nil {
			fmt.Printf("failed to execute command, %v", err)
			os.Exit(8)
		}
	}

	println("Commands executed successfully")
}
