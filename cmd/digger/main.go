package main

import (
	"context"
	"digger/pkg/configuration"
	"digger/pkg/digger"
	"digger/pkg/gcp"
	dg_github "digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/utils"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v51/github"
)

func main() {
	githubRepositoryOwner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	if githubRepositoryOwner != "" {
		utils.SendUsageRecord(githubRepositoryOwner, "log", "initialize")
	} else {
		utils.SendUsageRecord("", "log", "non github initialisation")
	}

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
		reportErrorAndExit(githubRepositoryOwner, "GITHUB_TOKEN is not defined", 1)
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		reportErrorAndExit(githubRepositoryOwner, "GITHUB_CONTEXT is not defined", 2)
	}

	parsedGhContext, err := models.GetGitHubContext(ghContext)
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to parse GitHub context. %s", err), 3)
	}
	println("GitHub context parsed successfully")

	walker := configuration.FileSystemDirWalker{}

	diggerConfig, err := configuration.NewDiggerConfig("./", &walker)
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	lock, err := utils.GetLock()
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to create lock provider. %s", err), 5)
	}
	println("Lock provider has been created successfully")

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := dg_github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	impactedProjects, requestedProject, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
	}
	logImpactedAndRequestedProjects(impactedProjects, requestedProject, prNumber)
	println("GitHub event processed successfully")

	if digger.CheckIfHelpComment(ghEvent) {
		reply := utils.GetCommands()
		githubPrService.PublishComment(prNumber, reply)
	}

	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
	}
	println("GitHub event converted to commands successfully")
	logCommands(commandsToRunPerProject)

	planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, prNumber)

	allAppliesSuccess, err := digger.RunCommandsPerProject(commandsToRunPerProject, repoOwner, repositoryName, eventName, prNumber, githubPrService, lock, planStorage, "")
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}

	if digger.CheckIfApplyComment(ghEvent) && diggerConfig.AutoMerge && allAppliesSuccess && (requestedProject == "" || len(impactedProjects) == 1) {
		digger.MergePullRequest(githubPrService, prNumber)
		println("PR merged successfully")
	}

	println("Commands executed successfully")

	reportErrorAndExit(githubRepositoryOwner, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()

}

func newPlanStorage(ghToken string, repoOwner string, repositoryName string, prNumber int) utils.PlanStorage {
	var planStorage utils.PlanStorage

	if os.Getenv("PLAN_UPLOAD_DESTINATION") == "github" {
		zipManager := utils.Zipper{}
		planStorage = &utils.GithubPlanStorage{
			Client:            github.NewTokenClient(context.Background(), ghToken),
			Owner:             repoOwner,
			RepoName:          repositoryName,
			PullRequestNumber: prNumber,
			ZipManager:        zipManager,
		}
	} else if os.Getenv("PLAN_UPLOAD_DESTINATION") == "gcp" {
		ctx, client := gcp.GetGoogleStorageClient()

		bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_BUCKET"))
		if bucketName == "" {
			reportErrorAndExit(repoOwner, fmt.Sprintf("GOOGLE_STORAGE_BUCKET is not defined"), 9)
		}
		bucket := client.Bucket(bucketName)
		planStorage = &utils.PlanStorageGcp{
			Client:  client,
			Bucket:  bucket,
			Context: ctx,
		}
	}
	return planStorage
}

func logImpactedAndRequestedProjects(projects []configuration.Project, requestedProject string, prNumber int) {
	if requestedProject != "" {
		log.Printf("The project '%s' was requested in pull request #%d comment\n", requestedProject, prNumber)
	}

	logMessage := fmt.Sprintf("Following projects are impacted by pull request #%d\n", prNumber)
	for _, p := range projects {
		logMessage += fmt.Sprintf("%s\n", p.Name)
	}
	log.Print(logMessage)
}

func logCommands(projectCommands []digger.ProjectCommand) {
	logMessage := fmt.Sprintf("Following commands are going to be executed:\n")
	for _, pc := range projectCommands {
		logMessage += fmt.Sprintf("project: %s: commands: ", pc.ProjectName)
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
