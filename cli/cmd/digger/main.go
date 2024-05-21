package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/samber/lo"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/diggerhq/digger/cli/pkg/comment_updater"
	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	core_reporting "github.com/diggerhq/digger/cli/pkg/core/reporting"
	"github.com/diggerhq/digger/cli/pkg/drift"

	"github.com/diggerhq/digger/cli/pkg/azure"
	"github.com/diggerhq/digger/cli/pkg/bitbucket"
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_locking "github.com/diggerhq/digger/cli/pkg/core/locking"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	core_storage "github.com/diggerhq/digger/cli/pkg/core/storage"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/gcp"
	github_models "github.com/diggerhq/digger/cli/pkg/github/models"
	"github.com/diggerhq/digger/cli/pkg/gitlab"
	"github.com/diggerhq/digger/cli/pkg/reporting"
	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	orchestrator "github.com/diggerhq/digger/libs/orchestrator"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"

	"gopkg.in/yaml.v3"

	"github.com/google/go-github/v61/github"
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

	repoOwner, repositoryName := utils.ParseRepoNamespace(ghRepository)
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
			CommentId string `json:"comment_id"`
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
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed unmarshall job string: %v", err), 4)
		}
		commentId64, err := strconv.ParseInt(inputs.CommentId, 10, 64)

		if job.BackendHostname != "" && job.BackendOrganisationName != "" && job.BackendJobToken != "" {
			log.Printf("Found settings sent by backend in job string, overriding backendApi and policyCheckecd r. setting: (orgName: %v BackedHost: %v token: %v)", job.BackendOrganisationName, job.BackendHostname, "****")
			backendApi = NewBackendApi(job.BackendHostname, job.BackendJobToken)
			policyChecker = NewPolicyChecker(job.BackendHostname, job.BackendOrganisationName, job.BackendJobToken)
		} else {
			reportErrorAndExit(githubActor, fmt.Sprintf("Missing values from job spec: hostname, orgName, token: %v %v", job.BackendHostname, job.BackendOrganisationName), 4)
		}

		err = githubPrService.SetOutput(*job.PullRequestNumber, "DIGGER_PR_NUMBER", fmt.Sprintf("%v", *job.PullRequestNumber))
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to set job output. Exiting. %s", err), 4)
		}

		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
		}

		serializedBatch, err := backendApi.ReportProjectJobStatus(repoName, job.ProjectName, inputs.Id, "started", time.Now(), nil, "")
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to report job status to backend. Exiting. %s", err), 4)
		}

		commentUpdater := comment_updater.BasicCommentUpdater{}

		commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, &githubPrService, commentId64)
		digger.UpdateAggregateStatus(serializedBatch, &githubPrService)

		planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, job.PullRequestNumber)

		log.Printf("Warn: Overriding commenting strategy to Comments-per-run")

		strategy := &reporting.CommentPerRunStrategy{
			Project:   job.ProjectName,
			IsPlan:    job.IsPlan(),
			TimeOfRun: time.Now(),
		}
		cireporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          *job.PullRequestNumber,
			ReportStrategy:    strategy,
			IsSupportMarkdown: true,
		}
		// using lazy reporter to be able to suppress empty plans
		reporter := reporting.NewCiReporterLazy(*cireporter)

		if err != nil {
			serializedBatch, reportingError := backendApi.ReportProjectJobStatus(repoName, job.ProjectName, inputs.Id, "failed", time.Now(), nil, "")
			if reportingError != nil {
				log.Printf("Failed to report job status to backend. %v", reportingError)
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed run commands. %s", err), 5)
			}
			commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, &githubPrService, commentId64)
			digger.UpdateAggregateStatus(serializedBatch, &githubPrService)

			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 5)
		}

		diggerConfig, _, _, err := digger_config.LoadDiggerConfig("./", false)
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
		}
		log.Printf("Digger digger_config read successfully\n")

		// Override the values of StateEnvVars and CommandEnvVars from workflow value_from values
		workflow := diggerConfig.Workflows[job.ProjectName]
		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)
		job.StateEnvVars = lo.Assign(job.StateEnvVars, stateEnvVars)
		job.CommandEnvVars = lo.Assign(job.CommandEnvVars, commandEnvVars)

		jobs := []orchestrator.Job{orchestrator.JsonToJob(job)}

		allAppliesSuccess, _, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, commentUpdater, backendApi, inputs.Id, true, commentId64, currentDir)
		if !allAppliesSuccess || err != nil {
			serializedBatch, reportingError := backendApi.ReportProjectJobStatus(repoName, job.ProjectName, inputs.Id, "failed", time.Now(), nil, "")
			if reportingError != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed run commands. %s", err), 5)
			}
			commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, &githubPrService, commentId64)
			digger.UpdateAggregateStatus(serializedBatch, &githubPrService)
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 5)
		}
		reportErrorAndExit(githubActor, "Digger finished successfully", 0)
	}

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig("./", true)
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
		err := digger.RunJob(jobs, ghRepository, githubActor, &githubPrService, policyChecker, planStorage, backendApi, nil, currentDir)
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

			StateEnvProvider, CommandEnvProvider := orchestrator.GetStateAndCommandProviders(projectConfig)

			job := orchestrator.Job{
				ProjectName:        projectConfig.Name,
				ProjectDir:         projectConfig.Dir,
				ProjectWorkspace:   projectConfig.Workspace,
				Terragrunt:         projectConfig.Terragrunt,
				OpenTofu:           projectConfig.OpenTofu,
				Commands:           []string{"digger drift-detect"},
				ApplyStage:         orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:          orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				RequestedBy:        githubActor,
				Namespace:          ghRepository,
				EventName:          "drift-detect",
				StateEnvProvider:   StateEnvProvider,
				CommandEnvProvider: CommandEnvProvider,
			}

			slackNotificationUrl := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL")
			var notification core_drift.Notification
			if slackNotificationUrl != "" {
				notification = drift.SlackNotification{slackNotificationUrl}
			} else {
				reportErrorAndExit(githubActor, fmt.Sprintf("Could not identify drift mode, please specify slack webhook url"), 8)
			}

			err := digger.RunJob(job, ghRepository, githubActor, &githubPrService, policyChecker, nil, backendApi, &notification, currentDir)
			if err != nil {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		}
	} else {

		impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
		if err != nil {
			if errors.Is(err, dg_github.UnhandledMergeGroupEventError) {
				reportErrorAndExit(githubActor, fmt.Sprintf("Graceful handling of GitHub event. %s", err), 0)
			} else {
				reportErrorAndExit(githubActor, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
			}
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

		err = githubPrService.SetOutput(prNumber, "DIGGER_PR_NUMBER", fmt.Sprintf("%v", prNumber))
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to set job output. Exiting. %s", err), 4)
		}

		planStorage := newPlanStorage(ghToken, repoOwner, repositoryName, githubActor, &prNumber)

		reporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          prNumber,
			ReportStrategy:    reportingStrategy,
			IsSupportMarkdown: true,
		}

		jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

		allAppliesSuccessful, atLeastOneApply, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, 0, currentDir)
		if err != nil {
			reportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			// aggregate status checks: failure
			if allAppliesSuccessful {
				if atLeastOneApply {
					githubPrService.SetStatus(prNumber, "failure", "digger/apply")
				} else {
					githubPrService.SetStatus(prNumber, "failure", "digger/plan")
				}
			}
		}

		if diggerConfig.AutoMerge && allAppliesSuccessful && atLeastOneApply && coversAllImpactedProjects {
			digger.MergePullRequest(&githubPrService, prNumber)
			log.Println("PR merged successfully")
		}

		if allAppliesSuccessful {
			// aggreate status checks: success
			if atLeastOneApply {
				githubPrService.SetStatus(prNumber, "success", "digger/apply")
			} else {
				githubPrService.SetStatus(prNumber, "success", "digger/plan")
			}
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

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig(currentDir, true)
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
	allAppliesSuccess, atLeastOneApply, err := digger.RunJobs(jobs, gitlabService, gitlabService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, 0, currentDir)

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

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig(currentDir, true)
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
	allAppliesSuccess, atLeastOneApply, err := digger.RunJobs(jobs, azureService, azureService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, 0, currentDir)
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

	diggerConfig, _, dependencyGraph, err := digger_config.LoadDiggerConfig("./", true)
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
		err := digger.RunJob(jobs, repository, actor, &bitbucketService, policyChecker, planStorage, backendApi, nil, currentDir)
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

			StateEnvProvider, CommandEnvProvider := orchestrator.GetStateAndCommandProviders(projectConfig)

			job := orchestrator.Job{
				ProjectName:        projectConfig.Name,
				ProjectDir:         projectConfig.Dir,
				ProjectWorkspace:   projectConfig.Workspace,
				Terragrunt:         projectConfig.Terragrunt,
				OpenTofu:           projectConfig.OpenTofu,
				Commands:           []string{"digger drift-detect"},
				ApplyStage:         orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:          orchestrator.ToConfigStage(workflow.Plan),
				CommandEnvVars:     commandEnvVars,
				StateEnvVars:       stateEnvVars,
				RequestedBy:        actor,
				Namespace:          repository,
				EventName:          "drift-detect",
				CommandEnvProvider: CommandEnvProvider,
				StateEnvProvider:   StateEnvProvider,
			}
			err := digger.RunJob(job, repository, actor, &bitbucketService, policyChecker, nil, backendApi, nil, currentDir)
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
				err := digger.RunJob(job, repository, actor, &bitbucketService, policyChecker, nil, backendApi, nil, currentDir)
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
				err := digger.RunJob(job, repository, actor, &bitbucketService, policyChecker, nil, backendApi, nil, currentDir)
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

			_, _, err = digger.RunJobs(jobs, &bitbucketService, &bitbucketService, lock, &reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, 0, currentDir)
			if err != nil {
				reportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		} else {
			reportErrorAndExit(actor, "Failed to detect running mode", 1)
		}

	}

	reportErrorAndExit(actor, "Digger finished successfully", 0)
}

func exec(actor string, projectName string, repoNamespace string, command string, prNumber int, lock core_locking.Lock, policyChecker core_policy.Checker, prService orchestrator.PullRequestService, orgService orchestrator.OrgService, reporter core_reporting.Reporter, backendApi core_backend.Api) {

	//SCMOrganisation, SCMrepository := utils.ParseRepoNamespace(runConfig.RepoNamespace)
	currentDir, err := os.Getwd()
	if err != nil {

		reportErrorAndExit(actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)

	}

	planStorage := newPlanStorage("", "", "", actor, nil)

	diggerConfig, _, dependencyGraph, err := digger_config.LoadDiggerConfig("./", true)
	if err != nil {
		reportErrorAndExit(actor, fmt.Sprintf("Failed to load digger config. %s", err), 4)
	}
	//impactedProjects := diggerConfig.GetModifiedProjects(strings.Split(runConfig.FilesChanged, ","))
	impactedProjects := diggerConfig.GetProjects(projectName)
	jobs, _, err := orchestrator.ConvertProjectsToJobs(actor, repoNamespace, command, prNumber, impactedProjects, nil, diggerConfig.Workflows)
	if err != nil {
		reportErrorAndExit(actor, fmt.Sprintf("Failed to convert impacted projects to commands. %s", err), 4)
	}

	jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)
	_, _, err = digger.RunJobs(jobs, prService, orgService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, 123, currentDir)
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
	if len(os.Args) == 1 {
		os.Args = append([]string{os.Args[0]}, "default")
	}
	if err := rootCmd.Execute(); err != nil {
		reportErrorAndExit("", fmt.Sprintf("Error occured during command exec: %v", err), 8)
	}

}

func newPlanStorage(ghToken string, ghRepoOwner string, ghRepositoryName string, requestedBy string, prNumber *int) core_storage.PlanStorage {
	var planStorage core_storage.PlanStorage

	uploadDestination := strings.ToLower(os.Getenv("PLAN_UPLOAD_DESTINATION"))
	switch {
	case uploadDestination == "github":
		zipManager := utils.Zipper{}
		planStorage = &storage.GithubPlanStorage{
			Client:            github.NewTokenClient(context.Background(), ghToken),
			Owner:             ghRepoOwner,
			RepoName:          ghRepositoryName,
			PullRequestNumber: *prNumber,
			ZipManager:        zipManager,
		}
	case uploadDestination == "gcp":
		ctx, client := gcp.GetGoogleStorageClient()
		bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET"))
		if bucketName == "" {
			reportErrorAndExit(requestedBy, "GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET is not defined", 9)
		}
		bucket := client.Bucket(bucketName)
		planStorage = &storage.PlanStorageGcp{
			Client:  client,
			Bucket:  bucket,
			Context: ctx,
		}
	case uploadDestination == "aws":
		ctx, client, err := storage.GetAWSStorageClient()
		if err != nil {
			reportErrorAndExit(requestedBy, fmt.Sprintf("Failed to create AWS storage client: %s", err), 9)
		}
		bucketName := strings.ToLower(os.Getenv("AWS_S3_BUCKET"))
		if bucketName == "" {
			reportErrorAndExit(requestedBy, "AWS_S3_BUCKET is not defined", 9)
		}
		planStorage = &storage.PlanStorageAWS{
			Context: ctx,
			Client:  client,
			Bucket:  bucketName,
		}
	case uploadDestination == "gitlab":
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
