package main

import (
	"context"
	"digger/pkg/azure"
	"digger/pkg/backend"
	core_backend "digger/pkg/core/backend"
	core_locking "digger/pkg/core/locking"
	core_policy "digger/pkg/core/policy"
	core_storage "digger/pkg/core/storage"
	"digger/pkg/digger"
	"digger/pkg/gcp"
	github_models "digger/pkg/github/models"
	"digger/pkg/gitlab"
	"digger/pkg/locking"
	"digger/pkg/policy"
	"digger/pkg/reporting"
	"digger/pkg/storage"
	"digger/pkg/usage"
	"digger/pkg/utils"
	"encoding/json"
	"fmt"
	configuration "github.com/diggerhq/lib-digger-config"
	orchestrator "github.com/diggerhq/lib-orchestrator"
	dg_github "github.com/diggerhq/lib-orchestrator/github"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
)

func gitHubCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
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
	diggerConfig, diggerConfigYaml, dependencyGraph, err := configuration.LoadDiggerConfig("./")
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	lock, err = locking.GetLock()
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to create lock provider. %s", err), 5)
	}
	println("Lock provider has been created successfully")

	ghRepository := os.Getenv("GITHUB_REPOSITORY")

	if ghRepository == "" {
		reportErrorAndExit(githubActor, "GITHUB_REPOSITORY is not defined", 3)
	}

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Convert to string
	yamlStr := string(yamlData)
	diggerNamespace := strings.ReplaceAll(ghRepository, "/", "-")

	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(diggerNamespace, p.Name, yamlStr)
		if err != nil {
			fmt.Printf("Failed to report project %s. %s\n", p.Name, err)
		}
	}

	splitRepositoryName := strings.Split(ghRepository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	runningMode := os.Getenv("INPUT_DIGGER_MODE")

	if runningMode == "manual" {
		command := os.Getenv("INPUT_DIGGER_COMMAND")
		if command == "" {
			reportErrorAndExit(githubActor, "provide 'command' to run in 'manual' mode", 1)
		}
		project := os.Getenv("INPUT_DIGGER_PROJECT")
		if project == "" {
			reportErrorAndExit(githubActor, "provide 'project' to run in 'manual' mode", 2)
		}

		var projectConfig configuration.Project
		for _, projectConfig = range diggerConfig.Projects {
			if projectConfig.Name == project {
				break
			}
		}
		workflow := diggerConfig.Workflows[projectConfig.Workflow]

		stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)

		planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, nil)

		jobs := orchestrator.Job{
			ProjectName:       project,
			ProjectDir:        projectConfig.Dir,
			ProjectWorkspace:  projectConfig.Workspace,
			Terragrunt:        projectConfig.Terragrunt,
			Commands:          []string{command},
			ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
			PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
			PullRequestNumber: nil,
			EventName:         "manual_invocation",
			RequestedBy:       githubActor,
			Namespace:         ghRepository,
			StateEnvVars:      stateEnvVars,
			CommandEnvVars:    commandEnvVars,
		}
		err := digger.RunJob(jobs, ghRepository, githubActor, &githubPrService, policyChecker, planStorage, backendApi, currentDir)
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
		}
	} else if runningMode == "drift-detection" {

		for _, projectConfig := range diggerConfig.Projects {
			if !projectConfig.DriftDetection {
				continue
			}
			workflow := diggerConfig.Workflows[projectConfig.Workflow]

			stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)

			job := orchestrator.Job{
				ProjectName:      projectConfig.Name,
				ProjectDir:       projectConfig.Dir,
				ProjectWorkspace: projectConfig.Workspace,
				Terragrunt:       projectConfig.Terragrunt,
				Commands:         []string{"digger drift-detect"},
				ApplyStage:       orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:        orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:   commandEnvVars,
				StateEnvVars:     stateEnvVars,
				RequestedBy:      githubActor,
				Namespace:        ghRepository,
				EventName:        "drift-detect",
			}
			err := digger.RunJob(job, ghRepository, githubActor, &githubPrService, policyChecker, nil, backendApi, currentDir)
			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		}
	} else {
		parsedGhActionContext, err := github_models.GetGitHubContext(ghContext)
		parsedGhContext := parsedGhActionContext.ToEventPackage()
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse GitHub context. %s", err), 3)
		}
		println("GitHub context parsed successfully")

		ghEvent := parsedGhContext.Event

		if wdEvent, ok := ghEvent.(github.WorkflowDispatchEvent); ok {
			type Inputs struct {
				JobString string `json:"job"`
			}

			var inputs Inputs

			jobJson := wdEvent.Inputs

			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to marshal job json. %s", err), 4)
			}

			fmt.Printf("Job json: %s\n", jobJson)

			err = json.Unmarshal(jobJson, &inputs)

			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
			}

			var job orchestrator.JobJson

			err = json.Unmarshal([]byte(inputs.JobString), &job)

			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
			}
			planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, job.PullRequestNumber)

			reporter := &reporting.CiReporter{
				CiService:      &githubPrService,
				PrNumber:       *job.PullRequestNumber,
				ReportStrategy: reportingStrategy,
			}

			jobs := []orchestrator.Job{orchestrator.JsonToJob(job)}

			jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

			_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, backendApi, currentDir)
			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 5)
			}
		} else {

			impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
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

			jobs, coversAllImpactedProjects, err := dg_github.ConvertGithubEventToJobs(parsedGhContext, impactedProjects, requestedProject, diggerConfig.Workflows)
			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
			}
			println("GitHub event converted to commands successfully")
			logCommands(jobs)

			planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, &prNumber)

			reporter := &reporting.CiReporter{
				CiService:      &githubPrService,
				PrNumber:       prNumber,
				ReportStrategy: reportingStrategy,
			}

			jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

			allAppliesSuccessful, atLeastOneApply, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, backendApi, currentDir)
			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}

			if diggerConfig.AutoMerge && allAppliesSuccessful && atLeastOneApply && coversAllImpactedProjects {
				digger.MergePullRequest(&githubPrService, prNumber)
				println("PR merged successfully")
			}
		}

		println("Commands executed successfully")
	}

	reportErrorAndExit(githubActor, "Digger finished successfully", 0)

	defer func() {
		if r := recover(); r != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Panic occurred. %s", r), 1)
		}
	}()
}

func gitLabCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
	println("Using GitLab.")

	projectNamespace := os.Getenv("CI_PROJECT_NAMESPACE")
	projectName := os.Getenv("CI_PROJECT_NAME")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	if gitlabToken == "" {
		fmt.Println("GITLAB_TOKEN is empty")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	fmt.Printf("main: working dir: %s \n", currentDir)

	diggerConfig, diggerConfigYaml, dependencyGraph, err := configuration.LoadDiggerConfig(currentDir)
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	println("Digger config read successfully")

	gitLabContext, err := gitlab.ParseGitLabContext()
	if err != nil {
		fmt.Printf("failed to parse GitLab context. %s\n", err.Error())
		os.Exit(4)
	}

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Convert to string
	yamlStr := string(yamlData)
	diggerNamespace := strings.ReplaceAll(gitLabContext.ProjectNamespace, "/", "-")

	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(diggerNamespace, p.Name, yamlStr)
		if err != nil {
			fmt.Printf("Failed to report project %s. %s\n", p.Name, err)
		}
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

	jobs, coversAllImpactedProjects, err := gitlab.ConvertGitLabEventToCommands(gitlabEvent, gitLabContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		fmt.Printf("failed to convert event to command, %v", err)
		os.Exit(7)
	}
	println("GitLab event converted to commands successfully")

	println("Digger commands to be executed:")
	for _, v := range jobs {
		fmt.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
	}

	planStorage := newPlanStorage("", "", "", gitLabContext.GitlabUserName, gitLabContext.MergeRequestIId)
	reporter := &reporting.CiReporter{
		CiService:      gitlabService,
		PrNumber:       *gitLabContext.MergeRequestIId,
		ReportStrategy: reportingStrategy,
	}
	jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)
	allAppliesSuccess, atLeastOneApply, err := digger.RunJobs(jobs, gitlabService, gitlabService, lock, reporter, planStorage, policyChecker, backendApi, currentDir)

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

func azureCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
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

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	fmt.Printf("main: working dir: %s \n", currentDir)

	diggerConfig, diggerConfigYaml, dependencyGraph, err := configuration.LoadDiggerConfig(currentDir)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to read Digger config. %s", err), 4)
	}
	fmt.Println("Digger config read successfully")

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Convert to string
	yamlStr := string(yamlData)
	diggerNamespace := strings.ReplaceAll(parsedAzureContext.BaseUrl, "/", "-")

	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(diggerNamespace, p.Name, yamlStr)
		if err != nil {
			fmt.Printf("Failed to report project %s. %s\n", p.Name, err)
		}
	}

	azureService, err := azure.NewAzureReposService(azureToken, parsedAzureContext.BaseUrl, parsedAzureContext.ProjectName, parsedAzureContext.RepositoryId)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to initialise azure service. %s", err), 5)
	}

	impactedProjects, requestedProject, prNumber, err := azure.ProcessAzureReposEvent(parsedAzureContext.Event, diggerConfig, azureService)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to process Azure event. %s", err), 6)
	}
	fmt.Println("Azure event processed successfully")

	jobs, coversAllImpactedProjects, err := azure.ConvertAzureEventToCommands(parsedAzureContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to convert event to command. %s", err), 7)

	}
	fmt.Println(fmt.Sprintf("Azure event converted to commands successfully: %v", jobs))

	for _, v := range jobs {
		fmt.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
	}

	var planStorage core_storage.PlanStorage

	reporter := &reporting.CiReporter{
		CiService:      azureService,
		PrNumber:       prNumber,
		ReportStrategy: reportingStrategy,
	}
	jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)
	allAppliesSuccess, atLeastOneApply, err := digger.RunJobs(jobs, azureService, azureService, lock, reporter, planStorage, policyChecker, backendApi, currentDir)
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
	var backendApi core_backend.Api
	if os.Getenv("DIGGER_TOKEN") != "" {
		if os.Getenv("DIGGER_ORGANISATION") == "" {
			log.Fatalf("Token specified but missing organisation: DIGGER_ORGANISATION. Please set this value in action configuration.")
		}
		policyChecker = policy.DiggerPolicyChecker{
			PolicyProvider: &policy.DiggerHttpPolicyProvider{
				DiggerHost:         os.Getenv("DIGGER_HOSTNAME"),
				DiggerOrganisation: os.Getenv("DIGGER_ORGANISATION"),
				AuthToken:          os.Getenv("DIGGER_TOKEN"),
				HttpClient:         http.DefaultClient,
			}}
		backendApi = backend.DiggerApi{
			DiggerHost: os.Getenv("DIGGER_HOSTNAME"),
			AuthToken:  os.Getenv("DIGGER_TOKEN"),
			HttpClient: http.DefaultClient,
		}
	} else {
		policyChecker = policy.NoOpPolicyChecker{}
		backendApi = backend.NoopApi{}
	}

	var reportStrategy reporting.ReportStrategy

	if os.Getenv("REPORTING_STRATEGY") == "comments_per_run" || os.Getenv("ACCUMULATE_PLANS") == "true" {
		reportStrategy = &reporting.CommentPerRunStrategy{
			TimeOfRun: time.Now(),
		}
	} else if os.Getenv("REPORTING_STRATEGY") == "latest_run_comment" {
		reportStrategy = &reporting.LatestRunCommentStrategy{
			TimeOfRun: time.Now(),
		}
	} else {
		reportStrategy = &reporting.MultipleCommentsStrategy{}
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
		gitHubCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.GitLab:
		gitLabCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.Azure:
		azureCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.BitBucket:
		print("Bitbucket support is currently in progress. If you would like to prioritise it, give this issue a bump: https://github.com/diggerhq/digger/issues/81")
	case digger.None:
		print("No CI detected.")
		os.Exit(10)
	}
}

func newPlanStorage(ghToken string, ghRepoOwner string, ghRepositoryName string, requestedBy string, prNumber *int) core_storage.PlanStorage {
	var planStorage core_storage.PlanStorage

	uploadDestination := strings.ToLower(os.Getenv("PLAN_UPLOAD_DESTINATION"))
	if uploadDestination == "github" {
		zipManager := utils.Zipper{}
		planStorage = &storage.GithubPlanStorage{
			Client:            github.NewTokenClient(context.Background(), ghToken),
			Owner:             ghRepoOwner,
			RepoName:          ghRepositoryName,
			PullRequestNumber: *prNumber,
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

func logCommands(projectCommands []orchestrator.Job) {
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
