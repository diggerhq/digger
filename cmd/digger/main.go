package main

import (
	"context"
	"digger/pkg/azure"
	"digger/pkg/configuration"
	"digger/pkg/digger"
	"digger/pkg/gcp"
	dg_github "digger/pkg/github"
	github_models "digger/pkg/github/models"
	"digger/pkg/gitlab"
	"digger/pkg/locking"
	"digger/pkg/models"
	"digger/pkg/storage"
	"digger/pkg/usage"
	"digger/pkg/utils"
	"fmt"
	"github.com/google/go-github/v51/github"
	"log"
	"os"
	"strings"
)

func gitHubCI(lock locking.Lock) {
	println("Using GitHub.")
	githubRepositoryOwner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	if githubRepositoryOwner != "" {
		usage.SendUsageRecord(githubRepositoryOwner, "log", "initialize")
	} else {
		usage.SendUsageRecord("", "log", "non github initialisation")
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		reportErrorAndExit(githubRepositoryOwner, "GITHUB_TOKEN is not defined", 1)
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		reportErrorAndExit(githubRepositoryOwner, "GITHUB_CONTEXT is not defined", 2)
	}

	parsedGhContext, err := github_models.GetGitHubContext(ghContext)
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

	lock, err = locking.GetLock()
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to create lock provider. %s", err), 5)
	}
	println("Lock provider has been created successfully")

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := dg_github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	if err != nil {
		reportErrorAndExit(githubRepositoryOwner, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
	}
	logImpactedProjects(impactedProjects, prNumber)
	println("GitHub event processed successfully")

	if dg_github.CheckIfHelpComment(ghEvent) {
		reply := utils.GetCommands()
		err := githubPrService.PublishComment(prNumber, reply)
		if err != nil {
			reportErrorAndExit(githubRepositoryOwner, "Failed to publish help command output", 1)
		}
	}

	commandsToRunPerProject, coversAllImpactedProjects, err := dg_github.ConvertGithubEventToCommands(ghEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
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

	if diggerConfig.AutoMerge && allAppliesSuccess && coversAllImpactedProjects {
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

func gitLabCI(lock locking.Lock) {
	println("Using GitLab.")
	projectNamespace := os.Getenv("CI_PROJECT_NAMESPACE")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	if gitlabToken == "" {
		fmt.Println("GITLAB_TOKEN is empty")
	}

	walker := configuration.FileSystemDirWalker{}
	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	fmt.Printf("main: working dir: %s \n", currentDir)

	diggerConfig, err := configuration.NewDiggerConfig(currentDir, &walker)
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	gitLabContext, err := gitlab.ParseGitLabContext()
	if err != nil {
		fmt.Printf("failed to parse GitLab context. %s\n", err.Error())
		os.Exit(4)
	}

	gitlabService, err := gitlab.NewGitLabService(gitlabToken, gitLabContext)
	if err != nil {
		fmt.Printf("failed to initialise GitLab service, %v", err)
		os.Exit(4)
	}

	gitlabEvent := gitlab.GitLabEvent{EventType: gitLabContext.EventType}

	impactedProjects, err := gitlab.ProcessGitLabEvent(gitLabContext, diggerConfig, gitlabService)
	if err != nil {
		fmt.Printf("failed to process GitLab event, %v", err)
		os.Exit(6)
	}
	println("GitLab event processed successfully")

	commandsToRunPerProject, err := gitlab.ConvertGitLabEventToCommands(gitlabEvent, gitLabContext, impactedProjects, diggerConfig.Workflows)
	if err != nil {
		fmt.Printf("failed to convert event to command, %v", err)
		os.Exit(7)
	}
	println("GitLab event converted to commands successfully")

	for _, v := range commandsToRunPerProject {
		fmt.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
	}

	//planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, prNumber)
	var planStorage storage.PlanStorage

	result, err := gitlab.RunCommandsPerProject(commandsToRunPerProject, *gitLabContext, diggerConfig, gitlabService, lock, planStorage, currentDir)
	if err != nil {
		fmt.Printf("failed to execute command, %v", err)
		os.Exit(8)
	}
	print(result)

	println("Commands executed successfully")
}

func azureCI(lock locking.Lock) {
	fmt.Println("Using Azure.")
	azureContext := os.Getenv("AZURE_CONTEXT")
	azureToken := os.Getenv("AZURE_TOKEN")
	if azureToken == "" {
		fmt.Println("AZURE_TOKEN is empty")
	}
	parsedAzureContext, err := azure.GetAzureReposContext(azureContext)
	if err != nil {
		fmt.Printf("failed to parse Azure context. %s\n", err.Error())
		os.Exit(4)
	}

	walker := configuration.FileSystemDirWalker{}
	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	fmt.Printf("main: working dir: %s \n", currentDir)

	diggerConfig, err := configuration.NewDiggerConfig(currentDir, &walker)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	azureService, err := azure.NewAzureReposService(azureToken, parsedAzureContext.BaseUrl, parsedAzureContext.ProjectName, parsedAzureContext.RepositoryId)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to initialise azure service. %s", err), 5)
	}

	impactedProjects, requestedProject, prNumber, err := azure.ProcessAzureReposEvent(parsedAzureContext.Event, diggerConfig, azureService)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to process Azure event. %s", err), 6)
	}
	println("Azure event processed successfully")

	commandsToRunPerProject, coversAllImpactedProjects, err := azure.ConvertAzureEventToCommands(parsedAzureContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to convert event to command. %s", err), 7)

	}
	println(fmt.Sprintf("Azure event converted to commands successfully: %v", commandsToRunPerProject))

	for _, v := range commandsToRunPerProject {
		fmt.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
	}

	var planStorage storage.PlanStorage

	allAppliesSuccess, err := digger.RunCommandsPerProject(commandsToRunPerProject, parsedAzureContext.ProjectName, parsedAzureContext.ProjectName, parsedAzureContext.EventType, prNumber, azureService, lock, planStorage, currentDir)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}

	if diggerConfig.AutoMerge && allAppliesSuccess && coversAllImpactedProjects {
		digger.MergePullRequest(azureService, prNumber)
		println("PR merged successfully")
	}

	println("Commands executed successfully")

	reportErrorAndExit(parsedAzureContext.BaseUrl, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()
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
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "version" {
		fmt.Println(utils.GetVersion())
		os.Exit(0)
	}
	if len(args) > 0 && args[0] == "help" {
		utils.DisplayCommands()
		os.Exit(0)
	}

	lock, err := locking.GetLock()
	if err != nil {
		fmt.Printf("Failed to create lock provider. %s\n", err)
		os.Exit(2)
	}
	println("Lock provider has been created successfully")

	ci := digger.DetectCI()
	switch ci {
	case digger.GitHub:
		gitHubCI(lock)
	case digger.GitLab:
		gitLabCI(lock)
	case digger.Azure:
		azureCI(lock)
	case digger.BitBucket:
	case digger.None:
		print("No CI detected.")
		os.Exit(10)
	}
}

func newPlanStorage(ghToken string, repoOwner string, repositoryName string, prNumber int) storage.PlanStorage {
	var planStorage storage.PlanStorage

	if os.Getenv("PLAN_UPLOAD_DESTINATION") == "github" {
		zipManager := utils.Zipper{}
		planStorage = &storage.GithubPlanStorage{
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
		planStorage = &storage.PlanStorageGcp{
			Client:  client,
			Bucket:  bucket,
			Context: ctx,
		}
	}
	return planStorage
}

func logImpactedProjects(projects []configuration.Project, prNumber int) {
	logMessage := fmt.Sprintf("Following projects are impacted by pull request #%d\n", prNumber)
	for _, p := range projects {
		logMessage += fmt.Sprintf("%s\n", p.Name)
	}
	log.Print(logMessage)
}

func logCommands(projectCommands []models.ProjectCommand) {
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
	println(message)
	err := usage.SendLogRecord(repoOwner, message)
	if err != nil {
		println("Failed to send log record. %s\n", err)
	}
	os.Exit(exitCode)
}
