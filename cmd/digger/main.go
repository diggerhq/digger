package main

import (
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/utils"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	utils.SendUsageRecord("", "log", "initialize")
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "version" {
		fmt.Println(utils.GetVersion())
		os.Exit(0)
	}
	if len(args) > 0 && args[0] == "help" {
		utils.DisplayCommands()
		os.Exit(0)
	}

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

	walker := digger.FileSystemDirWalker{}

	diggerConfig, err := digger.NewDiggerConfig("./", &walker)
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
	logImpactedProjects(impactedProjects, prNumber)
	println("GitHub event processed successfully")

	if digger.CheckIfHelpComment(ghEvent) {
		reply := utils.GetCommands()
		githubPrService.PublishComment(prNumber, reply)
	}

	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(repoOwner, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
	}
	logCommands(commandsToRunPerProject)
	println("GitHub event converted to commands successfully")

	allAppliesSuccess, err := digger.RunCommandsPerProject(commandsToRunPerProject, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, lock, "")
	if err != nil {
		reportErrorAndExit(repoOwner, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}

	if diggerConfig.AutoMerge && allAppliesSuccess {
		digger.MergePullRequest(githubPrService, prNumber)
		println("PR merged successfully")
	}

	println("Commands executed successfully")

	reportErrorAndExit(repoOwner, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(repoOwner, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()

}

func logImpactedProjects(projects []digger.Project, prNumber int) {
	logMessage := fmt.Sprintf("Following projects are impacted by pull request #%d\n", prNumber)
	for _, p := range projects {
		logMessage += fmt.Sprintf("%s\n", p.Name)
	}
	log.Print(logMessage)
}

func logCommands(projectCommands []digger.ProjectCommand) {
	logMessage := fmt.Sprintf("Following commands are going to be executed:")
	for _, pc := range projectCommands {
		logMessage += fmt.Sprintf("%s: ", pc.ProjectName)
		for _, c := range pc.Commands {
			logMessage += fmt.Sprintf("\"%s\", ", c)
		}
		logMessage += "\n"
	}
	log.Print(logMessage)
}

func reportErrorAndExit(repoOwner string, message string, exitCode int) {
	fmt.Printf(message)
	err := utils.SendLogRecord(repoOwner, message)
	if err != nil {
		fmt.Printf("Failed to send log record. %s\n", err)
	}
	os.Exit(exitCode)
}
