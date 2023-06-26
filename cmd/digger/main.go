package main

import (
	"context"
	"digger/pkg/azure"
	"digger/pkg/configuration"
	core_locking "digger/pkg/core/locking"
	"digger/pkg/core/models"
	core_policy "digger/pkg/core/policy"
	core_storage "digger/pkg/core/storage"
	"digger/pkg/digger"
	"digger/pkg/gcp"
	dg_github "digger/pkg/github"
	github_models "digger/pkg/github/models"
	"digger/pkg/gitlab"
	"digger/pkg/locking"
	"digger/pkg/policy"
	"digger/pkg/reporting"
	"digger/pkg/storage"
	"digger/pkg/usage"
	"digger/pkg/utils"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v53/github"
)

func gitHubCI(lock core_locking.Lock, policyChecker core_policy.Checker) {
	println("Using GitHub.")
	githubActor := os.Getenv("GITHUB_ACTOR")
	if githubActor != "" {
		usage.SendUsageRecord(githubActor, "log", "initialize")
	} else {
		usage.SendUsageRecord("", "log", "non github initialisation")
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		reportErrorAndExit(githubActor, "GITHUB_TOKEN is not defined", 1)
	}

	diggerGitHubToken := os.Getenv("DIGGER_GITHUB_TOKEN")
	if diggerGitHubToken != "" {
		fmt.Println("GITHUB_TOKEN has been overridden with DIGGER_GITHUB_TOKEN")
		ghToken = diggerGitHubToken
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		reportErrorAndExit(githubActor, "GITHUB_CONTEXT is not defined", 2)
	}

	parsedGhContext, err := github_models.GetGitHubContext(ghContext)
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse GitHub context. %s", err), 3)
	}
	println("GitHub context parsed successfully")

	walker := configuration.FileSystemDirWalker{}

	diggerConfig, dependencyGraph, err := configuration.LoadDiggerConfig("./", &walker)
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	lock, err = locking.GetLock()
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to create lock provider. %s", err), 5)
	}
	println("Lock provider has been created successfully")

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	splitRepositoryName := strings.Split(parsedGhContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)

	impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
	}
	logImpactedProjects(impactedProjects, prNumber)
	println("GitHub event processed successfully")

	if dg_github.CheckIfHelpComment(ghEvent) {
		reply := utils.GetCommands()
		err := githubPrService.PublishComment(prNumber, reply)
		if err != nil {
			reportErrorAndExit(githubActor, "Failed to publish help command output", 1)
		}
	}

	if len(impactedProjects) == 0 {
		reportErrorAndExit(githubActor, "No projects impacted", 0)
	}

	commandsToRunPerProject, coversAllImpactedProjects, err := dg_github.ConvertGithubEventToCommands(ghEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
	}
	println("GitHub event converted to commands successfully")
	logCommands(commandsToRunPerProject)

	planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, prNumber)

	reporter := &reporting.CiReporter{
		CiService: githubPrService,
		PrNumber:  prNumber,
	}
	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	allAppliesSuccessful, atLeastOneApply, err := digger.RunCommandsPerProject(commandsToRunPerProject, &dependencyGraph, parsedGhContext.Repository, githubActor, eventName, prNumber, githubPrService, lock, reporter, planStorage, policyChecker, currentDir)
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}

	if diggerConfig.AutoMerge && allAppliesSuccessful && atLeastOneApply && coversAllImpactedProjects {
		digger.MergePullRequest(githubPrService, prNumber)
		println("PR merged successfully")
	}

	println("Commands executed successfully")

	reportErrorAndExit(githubActor, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()
}

func gitLabCI(lock core_locking.Lock, policyChecker core_policy.Checker) {
	println("Using GitLab.")

	projectNamespace := os.Getenv("CI_PROJECT_NAMESPACE")
	projectName := os.Getenv("CI_PROJECT_NAME")
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

	diggerConfig, dependencyGraph, err := configuration.LoadDiggerConfig(currentDir, &walker)
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	gitLabContext, err := gitlab.ParseGitLabContext()
	if err != nil {
		fmt.Printf("failed to parse GitLab context. %s\n", err.Error())
		os.Exit(4)
	}

	// it's ok to not have merge request info if it has been merged
	if (gitLabContext.MergeRequestIId == nil || len(gitLabContext.OpenMergeRequests) == 0) && gitLabContext.EventType != "merge_request_merge" {
		fmt.Println("No merge request found.")
		os.Exit(0)
	}

	gitlabService, err := gitlab.NewGitLabService(gitlabToken, gitLabContext)
	if err != nil {
		fmt.Printf("failed to initialise GitLab service, %v", err)
		os.Exit(4)
	}

	gitlabEvent := gitlab.GitLabEvent{EventType: gitLabContext.EventType}

	impactedProjects, requestedProject, err := gitlab.ProcessGitLabEvent(gitLabContext, diggerConfig, gitlabService)
	if err != nil {
		fmt.Printf("failed to process GitLab event, %v", err)
		os.Exit(6)
	}
	println("GitLab event processed successfully")

	commandsToRunPerProject, coversAllImpactedProjects, err := gitlab.ConvertGitLabEventToCommands(gitlabEvent, gitLabContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		fmt.Printf("failed to convert event to command, %v", err)
		os.Exit(7)
	}
	println("GitLab event converted to commands successfully")

	println("Digger commands to be executed:")
	for _, v := range commandsToRunPerProject {
		fmt.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
	}

	diggerProjectNamespace := gitLabContext.ProjectNamespace + "/" + gitLabContext.ProjectName
	planStorage := newPlanStorage("", "", "", gitLabContext.GitlabUserName, *gitLabContext.MergeRequestIId)
	reporter := &reporting.CiReporter{
		CiService: gitlabService,
		PrNumber:  *gitLabContext.MergeRequestIId,
	}
	allAppliesSuccess, atLeastOneApply, err := digger.RunCommandsPerProject(commandsToRunPerProject, &dependencyGraph, diggerProjectNamespace, gitLabContext.GitlabUserName, gitLabContext.EventType.String(), *gitLabContext.MergeRequestIId, gitlabService, lock, reporter, planStorage, policyChecker, currentDir)

	if err != nil {
		fmt.Printf("failed to execute command, %v", err)
		os.Exit(8)
	}

	if diggerConfig.AutoMerge && atLeastOneApply && allAppliesSuccess && coversAllImpactedProjects {
		digger.MergePullRequest(gitlabService, *gitLabContext.MergeRequestIId)
		println("Merge request changes has been applied successfully")
	}

	println("Commands executed successfully")

	reportErrorAndExit(projectName, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(projectName, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()
}

func azureCI(lock core_locking.Lock, policyChecker core_policy.Checker) {
	fmt.Println("> Azure CI detected")
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

	diggerConfig, dependencyGraph, err := configuration.LoadDiggerConfig(currentDir, &walker)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	fmt.Println("Digger config read successfully")

	azureService, err := azure.NewAzureReposService(azureToken, parsedAzureContext.BaseUrl, parsedAzureContext.ProjectName, parsedAzureContext.RepositoryId)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to initialise azure service. %s", err), 5)
	}

	impactedProjects, requestedProject, prNumber, err := azure.ProcessAzureReposEvent(parsedAzureContext.Event, diggerConfig, azureService)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to process Azure event. %s", err), 6)
	}
	fmt.Println("Azure event processed successfully")

	commandsToRunPerProject, coversAllImpactedProjects, err := azure.ConvertAzureEventToCommands(parsedAzureContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to convert event to command. %s", err), 7)

	}
	fmt.Println(fmt.Sprintf("Azure event converted to commands successfully: %v", commandsToRunPerProject))

	for _, v := range commandsToRunPerProject {
		fmt.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
	}

	var planStorage core_storage.PlanStorage
	diggerProjectNamespace := parsedAzureContext.BaseUrl + "/" + parsedAzureContext.ProjectName

	reporter := &reporting.CiReporter{
		CiService: azureService,
		PrNumber:  prNumber,
	}
	allAppliesSuccess, atLeastOneApply, err := digger.RunCommandsPerProject(commandsToRunPerProject, &dependencyGraph, diggerProjectNamespace, parsedAzureContext.BaseUrl, parsedAzureContext.EventType, prNumber, azureService, lock, reporter, planStorage, policyChecker, currentDir)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}

	if diggerConfig.AutoMerge && allAppliesSuccess && atLeastOneApply && coversAllImpactedProjects {
		digger.MergePullRequest(azureService, prNumber)
		fmt.Println("PR merged successfully")
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
	var policyChecker core_policy.Checker
	if os.Getenv("DIGGER_TOKEN") != "" {
		policyChecker = policy.DiggerPolicyChecker{
			PolicyProvider: &policy.DiggerHttpPolicyProvider{
				DiggerHost: os.Getenv("DIGGER_HOSTNAME"),
				AuthToken:  os.Getenv("DIGGER_TOKEN"),
				HttpClient: http.DefaultClient,
			}}
	} else {
		policyChecker = policy.NoOpPolicyChecker{}
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
		gitHubCI(lock, policyChecker)
	case digger.GitLab:
		gitLabCI(lock, policyChecker)
	case digger.Azure:
		azureCI(lock, policyChecker)
	case digger.BitBucket:
	case digger.None:
		print("No CI detected.")
		os.Exit(10)
	}
}

func newPlanStorage(ghToken string, ghRepoOwner string, ghRepositoryName string, requestedBy string, prNumber int) core_storage.PlanStorage {
	var planStorage core_storage.PlanStorage

	uploadDestination := strings.ToLower(os.Getenv("PLAN_UPLOAD_DESTINATION"))
	if uploadDestination == "github" {
		zipManager := utils.Zipper{}
		planStorage = &storage.GithubPlanStorage{
			Client:            github.NewTokenClient(context.Background(), ghToken),
			Owner:             ghRepoOwner,
			RepoName:          ghRepositoryName,
			PullRequestNumber: prNumber,
			ZipManager:        zipManager,
		}
	} else if uploadDestination == "gcp" {
		ctx, client := gcp.GetGoogleStorageClient()
		bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_BUCKET"))
		if bucketName == "" {
			reportErrorAndExit(requestedBy, fmt.Sprintf("GOOGLE_STORAGE_BUCKET is not defined"), 9)
		}
		bucket := client.Bucket(bucketName)
		planStorage = &storage.PlanStorageGcp{
			Client:  client,
			Bucket:  bucket,
			Context: ctx,
		}
	} else if uploadDestination == "gitlab" {
		//TODO implement me
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
	fmt.Println(message)
	err := usage.SendLogRecord(repoOwner, message)
	if err != nil {
		fmt.Printf("Failed to send log record. %s\n", err)
	}
	os.Exit(exitCode)
}
