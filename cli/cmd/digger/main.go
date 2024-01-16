package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/diggerhq/digger/cli/pkg/azure"
	"github.com/diggerhq/digger/cli/pkg/backend"
	"github.com/diggerhq/digger/cli/pkg/bitbucket"
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	core_storage "github.com/diggerhq/digger/cli/pkg/core/storage"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/gcp"
	github_models "github.com/diggerhq/digger/cli/pkg/github/models"
	"github.com/diggerhq/digger/cli/pkg/gitlab"
	"github.com/diggerhq/digger/cli/pkg/locking"
	"github.com/diggerhq/digger/cli/pkg/policy"
	"github.com/diggerhq/digger/cli/pkg/reporting"
	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	orchestrator "github.com/diggerhq/digger/libs/orchestrator"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"

	"gopkg.in/yaml.v3"

	"github.com/google/go-github/v55/github"
)

func gitHubCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
	log.Printf("Using GitHub.\n")
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
		log.Println("GITHUB_TOKEN has been overridden with DIGGER_GITHUB_TOKEN")
		ghToken = diggerGitHubToken
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		reportErrorAndExit(githubActor, "GITHUB_CONTEXT is not defined", 2)
	}

	diggerOutPath := os.Getenv("DIGGER_OUT")
	if diggerOutPath == "" {
		diggerOutPath = os.Getenv("RUNNER_TEMP") + "/digger-out.log"
		os.Setenv("DIGGER_OUT", diggerOutPath)
	}

	runningMode := os.Getenv("INPUT_DIGGER_MODE")

	parsedGhActionContext, err := github_models.GetGitHubContext(ghContext)
	parsedGhContext := parsedGhActionContext.ToEventPackage()
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse GitHub context. %s", err), 3)
	}
	log.Printf("GitHub context parsed successfully\n")

	ghEvent := parsedGhContext.Event

	ghRepository := os.Getenv("GITHUB_REPOSITORY")

	if ghRepository == "" {
		reportErrorAndExit(githubActor, "GITHUB_REPOSITORY is not defined", 3)
	}

	splitRepositoryName := strings.Split(ghRepository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	// this is used when called from api by the backend and exits in the end of if statement
	if wdEvent, ok := ghEvent.(github.WorkflowDispatchEvent); ok && runningMode != "manual" && runningMode != "drift-detection" {
		type Inputs struct {
			JobString string `json:"job"`
			Id        string `json:"id"`
			CommentId int64  `json:"comment_id"`
		}

		var inputs Inputs

		jobJson := wdEvent.Inputs

		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to marshal job json. %s", err), 4)
		}

		err = json.Unmarshal(jobJson, &inputs)

		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
		}

		repoName := strings.ReplaceAll(ghRepository, "/", "-")

		var job orchestrator.JobJson

		err = json.Unmarshal([]byte(inputs.JobString), &job)

		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
		}

		err := backendApi.ReportProjectJobStatus(repoName, job.ProjectName, inputs.Id, "started", time.Now(), nil)

		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to report job status to backend. Exiting. %s", err), 4)
		}

		planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, job.PullRequestNumber)

		reporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          *job.PullRequestNumber,
			ReportStrategy:    reportingStrategy,
			IsSupportMarkdown: true,
		}

		// TOD Remove
		log.Printf("Received commentID: %v", inputs.CommentId)
		githubPrService.EditComment(*job.PullRequestNumber, inputs.CommentId, ":x: Edited by cli :x:")

		jobs := []orchestrator.Job{orchestrator.JsonToJob(job)}

		_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, backendApi, currentDir)
		if err != nil {
			reportingError := backendApi.ReportProjectJobStatus(repoName, job.ProjectName, inputs.Id, "failed", time.Now(), nil)
			if reportingError != nil {
				log.Printf("Failed to report job status to backend. %s", reportingError)
			}
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 5)
		}
		err = backendApi.ReportProjectJobStatus(repoName, job.ProjectName, inputs.Id, "succeeded", time.Now(),
			&core_backend.JobSummary{ResourcesCreated: 999, ResourcesUpdated: 998, ResourcesDeleted: 997})
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to report job status to backend. %s", err), 4)
		}
		reportErrorAndExit(githubActor, "Digger finished successfully", 0)
	}

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig("./")
	if err != nil {
		reportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Printf("Digger digger_config read successfully\n")

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Convert to string
	yamlStr := string(yamlData)
	repo := strings.ReplaceAll(ghRepository, "/", "-")

	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(repo, p.Name, yamlStr)
		if err != nil {
			log.Printf("Failed to report project %s. %s\n", p.Name, err)
		}
	}

	if runningMode == "manual" {
		command := os.Getenv("INPUT_DIGGER_COMMAND")
		if command == "" {
			reportErrorAndExit(githubActor, "provide 'command' to run in 'manual' mode", 1)
		}
		project := os.Getenv("INPUT_DIGGER_PROJECT")
		if project == "" {
			reportErrorAndExit(githubActor, "provide 'project' to run in 'manual' mode", 2)
		}

		var projectConfig digger_config.Project
		for _, projectConfig = range diggerConfig.Projects {
			if projectConfig.Name == project {
				break
			}
		}
		workflow := diggerConfig.Workflows[projectConfig.Workflow]

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

		planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, nil)

		jobs := orchestrator.Job{
			ProjectName:       project,
			ProjectDir:        projectConfig.Dir,
			ProjectWorkspace:  projectConfig.Workspace,
			Terragrunt:        projectConfig.Terragrunt,
			OpenTofu:          projectConfig.OpenTofu,
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

			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

			job := orchestrator.Job{
				ProjectName:      projectConfig.Name,
				ProjectDir:       projectConfig.Dir,
				ProjectWorkspace: projectConfig.Workspace,
				Terragrunt:       projectConfig.Terragrunt,
				OpenTofu:         projectConfig.OpenTofu,
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

		impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
		}
		impactedProjectsMsg := getImpactedProjectsAsString(impactedProjects, prNumber)
		log.Println(impactedProjectsMsg)
		log.Println("GitHub event processed successfully")

		if dg_github.CheckIfHelpComment(ghEvent) {
			reply := utils.GetCommands()
			_, err := githubPrService.PublishComment(prNumber, reply)
			if err != nil {
				reportErrorAndExit(githubActor, "Failed to publish help command output", 1)
			}
		}

		if dg_github.CheckIfShowProjectsComment(ghEvent) {
			reply := impactedProjectsMsg
			_, err := githubPrService.PublishComment(prNumber, reply)
			if err != nil {
				reportErrorAndExit(githubActor, "Failed to publish show-projects command output", 1)
			}
		}

		if len(impactedProjects) == 0 {
			reportErrorAndExit(githubActor, "No projects impacted", 0)
		}

		var jobs []orchestrator.Job
		coversAllImpactedProjects := false
		err = nil
		if prEvent, ok := ghEvent.(github.PullRequestEvent); ok {
			jobs, coversAllImpactedProjects, err = dg_github.ConvertGithubPullRequestEventToJobs(&prEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
		} else if commentEvent, ok := ghEvent.(github.IssueCommentEvent); ok {
			prBranchName, err := githubPrService.GetBranchName(*commentEvent.Issue.Number)
			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Error while retriving default branch from Issue: %v", err), 6)
			}
			jobs, coversAllImpactedProjects, err = dg_github.ConvertGithubIssueCommentEventToJobs(&commentEvent, impactedProjects, requestedProject, diggerConfig.Workflows, prBranchName)
		} else {
			reportErrorAndExit(githubActor, fmt.Sprintf("Unsupported GitHub event type. %s", err), 6)
		}

		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
		}
		log.Println("GitHub event converted to commands successfully")
		logCommands(jobs)

		planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, &prNumber)

		reporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          prNumber,
			ReportStrategy:    reportingStrategy,
			IsSupportMarkdown: true,
		}

		jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

		allAppliesSuccessful, atLeastOneApply, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, backendApi, currentDir)
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
		}

		if diggerConfig.AutoMerge && allAppliesSuccessful && atLeastOneApply && coversAllImpactedProjects {
			digger.MergePullRequest(&githubPrService, prNumber)
			log.Println("PR merged successfully")
		}

		log.Println("Commands executed successfully")
	}

	reportErrorAndExit(githubActor, "Digger finished successfully", 0)
}

func gitLabCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
	log.Println("Using GitLab.")

	projectNamespace := os.Getenv("CI_PROJECT_NAMESPACE")
	projectName := os.Getenv("CI_PROJECT_NAME")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	if gitlabToken == "" {
		log.Println("GITLAB_TOKEN is empty")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	log.Printf("main: working dir: %s \n", currentDir)

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig(currentDir)
	if err != nil {
		reportErrorAndExit(projectNamespace, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Println("Digger digger_config read successfully")

	gitLabContext, err := gitlab.ParseGitLabContext()
	if err != nil {
		log.Printf("failed to parse GitLab context. %s\n", err.Error())
		os.Exit(4)
	}

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Convert to string
	yamlStr := string(yamlData)
	repo := strings.ReplaceAll(gitLabContext.ProjectNamespace, "/", "-")

	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(repo, p.Name, yamlStr)
		if err != nil {
			log.Printf("Failed to report project %s. %s\n", p.Name, err)
		}
	}

	// it's ok to not have merge request info if it has been merged
	if (gitLabContext.MergeRequestIId == nil || len(gitLabContext.OpenMergeRequests) == 0) && gitLabContext.EventType != "merge_request_merge" {
		log.Println("No merge request found.")
		os.Exit(0)
	}

	gitlabService, err := gitlab.NewGitLabService(gitlabToken, gitLabContext)
	if err != nil {
		log.Printf("failed to initialise GitLab service, %v", err)
		os.Exit(4)
	}

	gitlabEvent := gitlab.GitLabEvent{EventType: gitLabContext.EventType}

	impactedProjects, requestedProject, err := gitlab.ProcessGitLabEvent(gitLabContext, diggerConfig, gitlabService)
	if err != nil {
		log.Printf("failed to process GitLab event, %v", err)
		os.Exit(6)
	}
	log.Println("GitLab event processed successfully")

	jobs, coversAllImpactedProjects, err := gitlab.ConvertGitLabEventToCommands(gitlabEvent, gitLabContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		log.Printf("failed to convert event to command, %v", err)
		os.Exit(7)
	}
	log.Println("GitLab event converted to commands successfully")

	log.Println("Digger commands to be executed:")
	for _, v := range jobs {
		log.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
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
		log.Printf("failed to execute command, %v", err)
		os.Exit(8)
	}

	if diggerConfig.AutoMerge && atLeastOneApply && allAppliesSuccess && coversAllImpactedProjects {
		digger.MergePullRequest(gitlabService, *gitLabContext.MergeRequestIId)
		log.Println("Merge request changes has been applied successfully")
	}

	log.Println("Commands executed successfully")

	reportErrorAndExit(projectName, "Digger finished successfully", 0)
}

func azureCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
	log.Println("> Azure CI detected")
	azureContext := os.Getenv("AZURE_CONTEXT")
	azureToken := os.Getenv("AZURE_TOKEN")
	if azureToken == "" {
		log.Println("AZURE_TOKEN is empty")
	}
	parsedAzureContext, err := azure.GetAzureReposContext(azureContext)
	if err != nil {
		log.Printf("failed to parse Azure context. %s\n", err.Error())
		os.Exit(4)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}
	log.Printf("main: working dir: %s \n", currentDir)

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig(currentDir)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Println("Digger digger_config read successfully")

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Convert to string
	yamlStr := string(yamlData)
	repo := strings.ReplaceAll(parsedAzureContext.BaseUrl, "/", "-")

	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(repo, p.Name, yamlStr)
		if err != nil {
			log.Printf("Failed to report project %s. %s\n", p.Name, err)
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
	log.Println("Azure event processed successfully")

	jobs, coversAllImpactedProjects, err := azure.ConvertAzureEventToCommands(parsedAzureContext, impactedProjects, requestedProject, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(parsedAzureContext.BaseUrl, fmt.Sprintf("Failed to convert event to command. %s", err), 7)

	}
	log.Println(fmt.Sprintf("Azure event converted to commands successfully: %v", jobs))

	for _, v := range jobs {
		log.Printf("command: %s, project: %s\n", strings.Join(v.Commands, ", "), v.ProjectName)
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
		log.Println("PR merged successfully")
	}

	log.Println("Commands executed successfully")

	reportErrorAndExit(parsedAzureContext.BaseUrl, "Digger finished successfully", 0)
}

func bitbucketCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy) {
	log.Printf("Using Bitbucket.\n")
	actor := os.Getenv("BITBUCKET_STEP_TRIGGERER_UUID")
	if actor != "" {
		usage.SendUsageRecord(actor, "log", "initialize")
	} else {
		usage.SendUsageRecord("", "log", "non github initialisation")
	}

	runningMode := os.Getenv("INPUT_DIGGER_MODE")

	repository := os.Getenv("BITBUCKET_REPO_FULL_NAME")

	if repository == "" {
		reportErrorAndExit(actor, "BITBUCKET_REPO_FULL_NAME is not defined", 3)
	}

	splitRepositoryName := strings.Split(repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]

	currentDir, err := os.Getwd()
	if err != nil {
		reportErrorAndExit(actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	diggerConfig, _, dependencyGraph, err := digger_config.LoadDiggerConfig("./")
	if err != nil {
		reportErrorAndExit(actor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Printf("Digger digger_config read successfully\n")

	authToken := os.Getenv("BITBUCKET_AUTH_TOKEN")

	if authToken == "" {
		reportErrorAndExit(actor, "BITBUCKET_AUTH_TOKEN is not defined", 3)
	}

	bitbucketService := bitbucket.BitbucketAPI{
		AuthToken:     authToken,
		HttpClient:    http.Client{},
		RepoWorkspace: repoOwner,
		RepoName:      repositoryName,
	}

	if runningMode == "manual" {
		command := os.Getenv("INPUT_DIGGER_COMMAND")
		if command == "" {
			reportErrorAndExit(actor, "provide 'command' to run in 'manual' mode", 1)
		}
		project := os.Getenv("INPUT_DIGGER_PROJECT")
		if project == "" {
			reportErrorAndExit(actor, "provide 'project' to run in 'manual' mode", 2)
		}

		var projectConfig digger_config.Project
		for _, projectConfig = range diggerConfig.Projects {
			if projectConfig.Name == project {
				break
			}
		}
		workflow := diggerConfig.Workflows[projectConfig.Workflow]

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

		planStorage := newPlanStorage("", repoOwner, repositoryName, actor, nil)

		jobs := orchestrator.Job{
			ProjectName:       project,
			ProjectDir:        projectConfig.Dir,
			ProjectWorkspace:  projectConfig.Workspace,
			Terragrunt:        projectConfig.Terragrunt,
			OpenTofu:          projectConfig.OpenTofu,
			Commands:          []string{command},
			ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
			PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
			PullRequestNumber: nil,
			EventName:         "manual_invocation",
			RequestedBy:       actor,
			Namespace:         repository,
			StateEnvVars:      stateEnvVars,
			CommandEnvVars:    commandEnvVars,
		}
		err := digger.RunJob(jobs, repository, actor, &bitbucketService, policyChecker, planStorage, backendApi, currentDir)
		if err != nil {
			reportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
		}
	} else if runningMode == "drift-detection" {

		for _, projectConfig := range diggerConfig.Projects {
			if !projectConfig.DriftDetection {
				continue
			}
			workflow := diggerConfig.Workflows[projectConfig.Workflow]

			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

			job := orchestrator.Job{
				ProjectName:      projectConfig.Name,
				ProjectDir:       projectConfig.Dir,
				ProjectWorkspace: projectConfig.Workspace,
				Terragrunt:       projectConfig.Terragrunt,
				OpenTofu:         projectConfig.OpenTofu,
				Commands:         []string{"digger drift-detect"},
				ApplyStage:       orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:        orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:   commandEnvVars,
				StateEnvVars:     stateEnvVars,
				RequestedBy:      actor,
				Namespace:        repository,
				EventName:        "drift-detect",
			}
			err := digger.RunJob(job, repository, actor, &bitbucketService, policyChecker, nil, backendApi, currentDir)
			if err != nil {
				reportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		}
	} else {
		var jobs []orchestrator.Job
		if os.Getenv("BITBUCKET_PR_ID") == "" && os.Getenv("BITBUCKET_BRANCH") == os.Getenv("DEFAULT_BRANCH") {
			for _, projectConfig := range diggerConfig.Projects {

				workflow := diggerConfig.Workflows[projectConfig.Workflow]
				log.Printf("workflow: %v", workflow)

				stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

				job := orchestrator.Job{
					ProjectName:      projectConfig.Name,
					ProjectDir:       projectConfig.Dir,
					ProjectWorkspace: projectConfig.Workspace,
					Terragrunt:       projectConfig.Terragrunt,
					OpenTofu:         projectConfig.OpenTofu,
					Commands:         workflow.Configuration.OnCommitToDefault,
					ApplyStage:       orchestrator.ToConfigStage(workflow.Apply),
					PlanStage:        orchestrator.ToConfigStage(workflow.Plan),
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
					RequestedBy:      actor,
					Namespace:        repository,
					EventName:        "commit_to_default",
				}
				err := digger.RunJob(job, repository, actor, &bitbucketService, policyChecker, nil, backendApi, currentDir)
				if err != nil {
					reportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
				}
			}
		} else if os.Getenv("BITBUCKET_PR_ID") == "" {
			for _, projectConfig := range diggerConfig.Projects {

				workflow := diggerConfig.Workflows[projectConfig.Workflow]

				stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

				job := orchestrator.Job{
					ProjectName:      projectConfig.Name,
					ProjectDir:       projectConfig.Dir,
					ProjectWorkspace: projectConfig.Workspace,
					Terragrunt:       projectConfig.Terragrunt,
					OpenTofu:         projectConfig.OpenTofu,
					Commands:         []string{"digger plan"},
					ApplyStage:       orchestrator.ToConfigStage(workflow.Apply),
					PlanStage:        orchestrator.ToConfigStage(workflow.Plan),
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
					RequestedBy:      actor,
					Namespace:        repository,
					EventName:        "commit_to_default",
				}
				err := digger.RunJob(job, repository, actor, &bitbucketService, policyChecker, nil, backendApi, currentDir)
				if err != nil {
					reportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
				}
			}
		} else if os.Getenv("BITBUCKET_PR_ID") != "" {
			prNumber, err := strconv.Atoi(os.Getenv("BITBUCKET_PR_ID"))
			if err != nil {
				reportErrorAndExit(actor, fmt.Sprintf("Failed to parse PR number. %s", err), 4)
			}
			impactedProjects, err := bitbucket.FindImpactedProjectsInBitbucket(diggerConfig, prNumber, &bitbucketService)

			if err != nil {
				reportErrorAndExit(actor, fmt.Sprintf("Failed to find impacted projects. %s", err), 5)
			}
			if len(impactedProjects) == 0 {
				reportErrorAndExit(actor, "No projects impacted", 0)
			}

			impactedProjectsMsg := getImpactedProjectsAsString(impactedProjects, prNumber)
			log.Println(impactedProjectsMsg)
			if err != nil {
				reportErrorAndExit(actor, fmt.Sprintf("Failed to find impacted projects. %s", err), 5)
			}

			for _, project := range impactedProjects {
				workflow := diggerConfig.Workflows[project.Workflow]

				stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

				job := orchestrator.Job{
					ProjectName:       project.Name,
					ProjectDir:        project.Dir,
					ProjectWorkspace:  project.Workspace,
					Terragrunt:        project.Terragrunt,
					OpenTofu:          project.OpenTofu,
					Commands:          workflow.Configuration.OnPullRequestPushed,
					ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
					PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
					CommandEnvVars:    commandEnvVars,
					StateEnvVars:      stateEnvVars,
					PullRequestNumber: &prNumber,
					RequestedBy:       actor,
					Namespace:         repository,
					EventName:         "pull_request",
				}
				jobs = append(jobs, job)
			}

			reporter := reporting.CiReporter{
				CiService:      &bitbucketService,
				PrNumber:       prNumber,
				ReportStrategy: reportingStrategy,
			}

			log.Println("Bitbucket trigger converted to commands successfully")

			logCommands(jobs)

			planStorage := newPlanStorage("", repoOwner, repositoryName, actor, nil)

			jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

			_, _, err = digger.RunJobs(jobs, &bitbucketService, &bitbucketService, lock, &reporter, planStorage, policyChecker, backendApi, currentDir)
			if err != nil {
				reportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		} else {
			reportErrorAndExit(actor, "Failed to detect running mode", 1)
		}

	}

	reportErrorAndExit(actor, "Digger finished successfully", 0)
}

/*
Exit codes:
0 - No errors
1 - Failed to read digger digger_config
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
		log.Println(utils.GetVersion())
		os.Exit(0)
	}
	if len(args) > 0 && args[0] == "help" {
		utils.DisplayCommands()
		os.Exit(0)
	}
	var policyChecker core_policy.Checker
	var backendApi core_backend.Api
	if os.Getenv("NO_BACKEND") == "true" {
		log.Println("WARNING: running in 'backendless' mode. Features that require backend will not be available.")
		policyChecker = policy.NoOpPolicyChecker{}
		backendApi = backend.NoopApi{}
	} else if os.Getenv("DIGGER_TOKEN") != "" {
		if os.Getenv("DIGGER_ORGANISATION") == "" {
			log.Fatalf("Token specified but missing organisation: DIGGER_ORGANISATION. Please set this value in action digger_config.")
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
		reportErrorAndExit("", "DIGGER_TOKEN not specified. You can get one at https://cloud.digger.dev, or self-manage a backend of Digger Community Edition (change DIGGER_HOSTNAME). You can also pass 'no-backend: true' option; in this case some of the features may not be available.", 1)
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
		log.Printf("Failed to create lock provider. %s\n", err)
		os.Exit(2)
	}
	log.Println("Lock provider has been created successfully")

	ci := digger.DetectCI()

	var logLeader = "Unknown CI"

	switch ci {
	case digger.GitHub:
		logLeader = os.Getenv("GITHUB_ACTOR")
		gitHubCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.GitLab:
		logLeader = os.Getenv("CI_PROJECT_NAME")
		gitLabCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.Azure:
		// This should be refactored in the future because in this way the parsing
		// is done twice, both here and inside azureCI, a better solution might be
		// to encapsulate it into a method on the azure package and then grab the
		// value here and pass it into the azureCI call.
		azureContext := os.Getenv("AZURE_CONTEXT")
		parsedAzureContext, _ := azure.GetAzureReposContext(azureContext)
		logLeader = parsedAzureContext.BaseUrl
		azureCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.BitBucket:
		logLeader = os.Getenv("BITBUCKET_STEP_TRIGGERER_UUID")
		bitbucketCI(lock, policyChecker, backendApi, reportStrategy)
	case digger.None:
		print("No CI detected.")
		os.Exit(10)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Println(fmt.Sprintf("stacktrace from panic: \n" + string(debug.Stack())))
			err := usage.SendLogRecord(logLeader, fmt.Sprintf("Panic occurred. %s", r))
			if err != nil {
				log.Printf("Failed to send log record. %s\n", err)
			}
			os.Exit(1)
		}
	}()
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

func getImpactedProjectsAsString(projects []digger_config.Project, prNumber int) string {
	msg := fmt.Sprintf("Following projects are impacted by pull request #%d\n", prNumber)
	for _, p := range projects {
		msg += fmt.Sprintf("- %s\n", p.Name)
	}
	return msg
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
	log.Println(message)
	err := usage.SendLogRecord(repoOwner, message)
	if err != nil {
		log.Printf("Failed to send log record. %s\n", err)
	}
	os.Exit(exitCode)
}

func init() {
	log.SetOutput(os.Stdout)

	if os.Getenv("DEBUG") == "true" {
		log.SetFlags(log.Ltime | log.Lshortfile)
	}
}
