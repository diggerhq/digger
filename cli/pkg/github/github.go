package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/backend"
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/drift"
	github_models "github.com/diggerhq/digger/cli/pkg/github/models"
	"github.com/diggerhq/digger/cli/pkg/policy"
	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	core_locking "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/orchestrator"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func GitHubCI(lock core_locking.Lock, policyChecker core_policy.Checker, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy, commentUpdaterProvider comment_updater.CommentUpdaterProvider, driftNotifcationProvider drift.DriftNotificationProvider) {
	log.Printf("Using GitHub.\n")
	githubActor := os.Getenv("GITHUB_ACTOR")
	if githubActor != "" {
		usage.SendUsageRecord(githubActor, "log", "initialize")
	} else {
		usage.SendUsageRecord("", "log", "non github initialisation")
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		usage.ReportErrorAndExit(githubActor, "GITHUB_TOKEN is not defined", 1)
	}

	diggerGitHubToken := os.Getenv("DIGGER_GITHUB_TOKEN")
	if diggerGitHubToken != "" {
		log.Println("GITHUB_TOKEN has been overridden with DIGGER_GITHUB_TOKEN")
		ghToken = diggerGitHubToken
	}

	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		usage.ReportErrorAndExit(githubActor, "GITHUB_CONTEXT is not defined", 2)
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
		usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse GitHub context. %s", err), 3)
	}
	log.Printf("GitHub context parsed successfully\n")

	ghEvent := parsedGhContext.Event

	ghRepository := os.Getenv("GITHUB_REPOSITORY")

	if ghRepository == "" {
		usage.ReportErrorAndExit(githubActor, "GITHUB_REPOSITORY is not defined", 3)
	}

	repoOwner, repositoryName := utils.ParseRepoNamespace(ghRepository)
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)

	currentDir, err := os.Getwd()
	if err != nil {
		usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	// this is used when called from api by the backend and exits in the end of if statement
	if wdEvent, ok := ghEvent.(github.WorkflowDispatchEvent); ok && runningMode != "manual" && runningMode != "drift-detection" {

		var inputs scheduler.WorkflowInput

		jobJson := wdEvent.Inputs

		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to marshal jobSpec json. %s", err), 4)
		}

		err = json.Unmarshal(jobJson, &inputs)

		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
		}

		repoName := strings.ReplaceAll(ghRepository, "/", "-")

		var jobSpec orchestrator.JobJson
		err = json.Unmarshal([]byte(inputs.JobString), &jobSpec)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed unmarshall jobSpec string: %v", err), 4)
		}
		commentId64, err := strconv.ParseInt(inputs.CommentId, 10, 64)

		if jobSpec.BackendHostname != "" && jobSpec.BackendOrganisationName != "" && jobSpec.BackendJobToken != "" {
			log.Printf("Found settings sent by backend in jobSpec string, overriding backendApi and policyCheckecd r. setting: (orgName: %v BackedHost: %v token: %v)", jobSpec.BackendOrganisationName, jobSpec.BackendHostname, "****")
			backendApi = backend.NewBackendApi(jobSpec.BackendHostname, jobSpec.BackendJobToken)
			policyChecker = policy.NewPolicyChecker(jobSpec.BackendHostname, jobSpec.BackendOrganisationName, jobSpec.BackendJobToken)
		} else {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Missing values from job spec: hostname, orgName, token: %v %v", jobSpec.BackendHostname, jobSpec.BackendOrganisationName), 4)
		}

		err = githubPrService.SetOutput(*jobSpec.PullRequestNumber, "DIGGER_PR_NUMBER", fmt.Sprintf("%v", *jobSpec.PullRequestNumber))
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to set jobSpec output. Exiting. %s", err), 4)
		}

		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to parse jobs json. %s", err), 4)
		}

		serializedBatch, err := backendApi.ReportProjectJobStatus(repoName, jobSpec.ProjectName, inputs.Id, "started", time.Now(), nil, "", "")
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to report jobSpec status to backend. Exiting. %s", err), 4)
		}

		files, err := githubPrService.GetChangedFiles(*jobSpec.PullRequestNumber)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("could not get changed files: %v", err), 4)
		}
		
		diggerConfig, _, _, err := digger_config.LoadDiggerConfig("./", false, files)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
		}
		log.Printf("Digger digger_config read successfully\n")

		log.Printf("Warn: Overriding commenting strategy to Comments-per-run")
		strategy := &reporting.CommentPerRunStrategy{
			Title:     fmt.Sprintf("%v for %v", jobSpec.JobType, jobSpec.ProjectName),
			TimeOfRun: time.Now(),
		}
		cireporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          *jobSpec.PullRequestNumber,
			ReportStrategy:    strategy,
			IsSupportMarkdown: true,
		}
		// using lazy reporter to be able to suppress empty plans
		var reporter reporting.Reporter = reporting.NewCiReporterLazy(*cireporter)

		reportTerraformOutput := false
		commentUpdater, err := commentUpdaterProvider.Get(*diggerConfig)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("could not get comment updater: %v", err), 8)
		}
		if diggerConfig.CommentRenderMode == digger_config.CommentRenderModeBasic {
		} else if diggerConfig.CommentRenderMode == digger_config.CommentRenderModeGroupByModule {
			reporter = reporting.NoopReporter{}
			reportTerraformOutput = true
		} else {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Unknown comment render mode found: %v", diggerConfig.CommentRenderMode), 8)
		}

		commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, &githubPrService, commentId64)
		digger.UpdateAggregateStatus(serializedBatch, &githubPrService)

		planStorage := storage.NewPlanStorage(ghToken, repoOwner, repositoryName, githubActor, jobSpec.PullRequestNumber)

		if err != nil {
			serializedBatch, reportingError := backendApi.ReportProjectJobStatus(repoName, jobSpec.ProjectName, inputs.Id, "failed", time.Now(), nil, "", "")
			if reportingError != nil {
				log.Printf("Failed to report jobSpec status to backend. %v", reportingError)
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed run commands. %s", err), 5)
			}
			commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, &githubPrService, commentId64)
			digger.UpdateAggregateStatus(serializedBatch, &githubPrService)

			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 5)
		}

		// Override the values of StateEnvVars and CommandEnvVars from workflow value_from values
		workflow := diggerConfig.Workflows[jobSpec.ProjectName]
		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)
		jobSpec.StateEnvVars = lo.Assign(jobSpec.StateEnvVars, stateEnvVars)
		jobSpec.CommandEnvVars = lo.Assign(jobSpec.CommandEnvVars, commandEnvVars)

		jobs := []orchestrator.Job{orchestrator.JsonToJob(jobSpec)}

		allAppliesSuccess, _, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, commentUpdater, backendApi, inputs.Id, true, reportTerraformOutput, commentId64, currentDir)
		if !allAppliesSuccess || err != nil {
			serializedBatch, reportingError := backendApi.ReportProjectJobStatus(repoName, jobSpec.ProjectName, inputs.Id, "failed", time.Now(), nil, "", "")
			if reportingError != nil {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed run commands. %s", err), 5)
			}
			commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, &githubPrService, commentId64)
			digger.UpdateAggregateStatus(serializedBatch, &githubPrService)
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 5)
		}
		usage.ReportErrorAndExit(githubActor, "Digger finished successfully", 0)
	}

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig("./", true, nil)
	if err != nil {
		usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
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
			usage.ReportErrorAndExit(githubActor, "provide 'command' to run in 'manual' mode", 1)
		}
		project := os.Getenv("INPUT_DIGGER_PROJECT")
		if project == "" {
			usage.ReportErrorAndExit(githubActor, "provide 'project' to run in 'manual' mode", 2)
		}

		var projectConfig digger_config.Project
		for _, projectConfig = range diggerConfig.Projects {
			if projectConfig.Name == project {
				break
			}
		}
		workflow := diggerConfig.Workflows[projectConfig.Workflow]

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)

		planStorage := storage.NewPlanStorage(ghToken, repoOwner, repositoryName, githubActor, nil)

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
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
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

			notification, err := driftNotifcationProvider.Get(githubPrService)
			if err != nil {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("could not get drift notification type: %v", err), 8)
			}

			err = digger.RunJob(job, ghRepository, githubActor, &githubPrService, policyChecker, nil, backendApi, &notification, currentDir)
			if err != nil {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
			}
		}
	} else {

		impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
		if err != nil {
			if errors.Is(err, dg_github.UnhandledMergeGroupEventError) {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Graceful handling of GitHub event. %s", err), 0)
			} else {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to process GitHub event. %s", err), 6)
			}
		}
		log.Printf("Following projects are impacted by pull request #%d\n", prNumber)
		for _, p := range impactedProjects {
			log.Printf("- %s\n", p.Name)
		}
		log.Println("GitHub event processed successfully")

		if dg_github.CheckIfHelpComment(ghEvent) {
			reply := utils.GetCommands()
			_, err := githubPrService.PublishComment(prNumber, reply)
			if err != nil {
				usage.ReportErrorAndExit(githubActor, "Failed to publish help command output", 1)
			}
		}

		if len(impactedProjects) == 0 {
			usage.ReportErrorAndExit(githubActor, "No projects impacted", 0)
		}

		var jobs []orchestrator.Job
		coversAllImpactedProjects := false
		err = nil
		if prEvent, ok := ghEvent.(github.PullRequestEvent); ok {
			jobs, coversAllImpactedProjects, err = dg_github.ConvertGithubPullRequestEventToJobs(&prEvent, impactedProjects, requestedProject, *diggerConfig)
		} else if commentEvent, ok := ghEvent.(github.IssueCommentEvent); ok {
			prBranchName, _, err := githubPrService.GetBranchName(*commentEvent.Issue.Number)
			if err != nil {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Error while retriving default branch from Issue: %v", err), 6)
			}
			jobs, coversAllImpactedProjects, err = dg_github.ConvertGithubIssueCommentEventToJobs(&commentEvent, impactedProjects, requestedProject, diggerConfig.Workflows, prBranchName)
		} else {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Unsupported GitHub event type. %s", err), 6)
		}

		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to convert GitHub event to commands. %s", err), 7)
		}
		log.Println("GitHub event converted to commands successfully")
		logCommands(jobs)

		err = githubPrService.SetOutput(prNumber, "DIGGER_PR_NUMBER", fmt.Sprintf("%v", prNumber))
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to set job output. Exiting. %s", err), 4)
		}

		planStorage := storage.NewPlanStorage(ghToken, repoOwner, repositoryName, githubActor, &prNumber)

		reporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          prNumber,
			ReportStrategy:    reportingStrategy,
			IsSupportMarkdown: true,
		}

		jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

		allAppliesSuccessful, atLeastOneApply, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, false, 0, currentDir)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
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

	usage.ReportErrorAndExit(githubActor, "Digger finished successfully", 0)
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
