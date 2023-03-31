package main

import (
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/gitlab"
	"digger/pkg/models"
	"digger/pkg/utils"
	"fmt"
	"os"
	"strings"
)

func gitHubCI(diggerConfig *digger.DiggerConfig, lock utils.Lock) {
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		println("GITHUB_TOKEN is not defined")
		os.Exit(3)
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		fmt.Printf("GITHUB_CONTEXT is not defined. \n")
		os.Exit(4)
	}

	parsedGhContext, err := models.GetGitHubContext(ghContext)
	if err != nil {
		fmt.Printf("failed to parse GitHub context. %s\n", err.Error())
		os.Exit(4)
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
	println("Commands executed successfully")
}

func gitLabCI(diggerConfig *digger.DiggerConfig, lock utils.Lock) {
	gitLabContext, err := gitlab.ParseGitLabContext()
	if err != nil {
		fmt.Printf("failed to parse GitLab context. %s\n", err.Error())
		os.Exit(4)
	}

	gitlabService := gitlab.NewGitLabService(gitLabContext.Token, gitLabContext.ProjectName, gitLabContext.ProjectNamespace)
	gitlabEvent := gitlab.GitLabEvent{Name: gitLabContext.PipelineSource.String()}

	impactedProjects, prNumber, err := gitlab.ProcessGitLabEvent(gitlabEvent, diggerConfig, gitlabService)
	if err != nil {
		fmt.Printf("failed to process GitLab event, %v", err)
		os.Exit(6)
	}
	println("GitHub event processed successfully")

	commandsToRunPerProject, err := gitlab.ConvertGitLabEventToCommands(gitlabEvent, impactedProjects)
	if err != nil {
		fmt.Printf("failed to convert event to command, %v", err)
		os.Exit(7)
	}
	println("GitHub event converted to commands successfully")

	err = gitlab.RunCommandsPerProject(commandsToRunPerProject, gitLabContext.ProjectNamespace, gitLabContext.ProjectName, gitlabEvent.Name, prNumber, diggerConfig, gitlabService, lock, "")
	if err != nil {
		fmt.Printf("failed to execute command, %v", err)
		os.Exit(8)
	}
	println("Commands executed successfully")
}

/*
Exit codes:
0 - No errors
1 - Failed to read digger config
2 - Failed to create lock provider
3 - Failed to find auth token
4 - Failed to initialise CI context
5 -
6 - failed to process CI event
7 - failed to convert event to command
8 - failed to execute command
10 - No CI detected
*/

func main() {
	diggerConfig, err := digger.NewDiggerConfig("")
	if err != nil {
		fmt.Printf("Failed to read digger config. %s\n", err)
		os.Exit(1)
	}
	println("Digger config read successfully")

	lock, err := utils.GetLock()
	if err != nil {
		fmt.Printf("Failed to create lock provider. %s\n", err)
		os.Exit(2)
	}
	println("Lock provider has been created successfully")

	ci := digger.DetectCI()
	switch ci {
	case digger.GitHub:
		gitHubCI(diggerConfig, lock)
	case digger.GitLab:
		gitLabCI(diggerConfig, lock)
	case digger.BitBucket:
	case digger.None:
		print("No CI detected.")
		os.Exit(10)
	}
}
