package github

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/drift"
	github_models "github.com/diggerhq/digger/cli/pkg/github/models"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/cli/pkg/utils"
	core_backend "github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	core_locking "github.com/diggerhq/digger/libs/locking"
	core_policy "github.com/diggerhq/digger/libs/policy"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/storage"
	"github.com/google/go-github/v61/github"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strings"
)

func GitHubCI(lock core_locking.Lock, policyCheckerProvider core_policy.PolicyCheckerProvider, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy, githubServiceProvider dg_github.GithubServiceProvider, commentUpdaterProvider comment_updater.CommentUpdaterProvider, driftNotifcationProvider drift.DriftNotificationProvider) {
	log.Printf("Using GitHub.\n")
	githubActor := os.Getenv("GITHUB_ACTOR")
	if githubActor != "" {
		usage.SendUsageRecord(githubActor, "log", "initialize")
	} else {
		usage.SendUsageRecord("", "log", "non github initialisation")
	}

	// default policy checker for backwards compatability, will be overriden in orchestrator flow
	hostName := os.Getenv("DIGGER_HOSTNAME")
	token := os.Getenv("DIGGER_TOKEN")
	orgName := os.Getenv("DIGGER_ORGANISATION")
	var policyChecker, _ = policyCheckerProvider.Get(hostName, token, orgName)

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
	githubPrService, err := githubServiceProvider.NewService(ghToken, repositoryName, repoOwner)
	if err != nil {
		usage.ReportErrorAndExit(githubActor, fmt.Sprintf("could not create pr service: %v", err), 4)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	diggerConfig, diggerConfigYaml, dependencyGraph, err := digger_config.LoadDiggerConfig("./", true, nil)
	if err != nil {
		usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Printf("Digger digger_config read successfully\n")

	if diggerConfig.PrLocks == false {
		log.Printf("info: Using noop lock as configured in digger.yml")
		lock = core_locking.NoOpLock{}
	}

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

		stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)

		planStorage, err := storage.NewPlanStorage(ghToken, repoOwner, repositoryName, nil)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to get plan storage. %s", err), 4)
		}

		jobs := scheduler.Job{
			ProjectName:       project,
			ProjectDir:        projectConfig.Dir,
			ProjectWorkspace:  projectConfig.Workspace,
			Terragrunt:        projectConfig.Terragrunt,
			OpenTofu:          projectConfig.OpenTofu,
			Commands:          []string{command},
			ApplyStage:        scheduler.ToConfigStage(workflow.Apply),
			PlanStage:         scheduler.ToConfigStage(workflow.Plan),
			PullRequestNumber: nil,
			EventName:         "manual_invocation",
			RequestedBy:       githubActor,
			Namespace:         ghRepository,
			StateEnvVars:      stateEnvVars,
			CommandEnvVars:    commandEnvVars,
		}
		err = digger.RunJob(jobs, ghRepository, githubActor, &githubPrService, policyChecker, planStorage, backendApi, nil, currentDir)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
		}
	} else if runningMode == "drift-detection" {

		for _, projectConfig := range diggerConfig.Projects {
			if !projectConfig.DriftDetection {
				continue
			}
			workflow := diggerConfig.Workflows[projectConfig.Workflow]

			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)

			StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(projectConfig)

			job := scheduler.Job{
				ProjectName:        projectConfig.Name,
				ProjectDir:         projectConfig.Dir,
				ProjectWorkspace:   projectConfig.Workspace,
				Terragrunt:         projectConfig.Terragrunt,
				OpenTofu:           projectConfig.OpenTofu,
				Commands:           []string{"digger drift-detect"},
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
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

		var jobs []scheduler.Job
		coversAllImpactedProjects := false
		err = nil
		if prEvent, ok := ghEvent.(github.PullRequestEvent); ok {
			jobs, coversAllImpactedProjects, err = dg_github.ConvertGithubPullRequestEventToJobs(&prEvent, impactedProjects, requestedProject, *diggerConfig, true)
		} else if commentEvent, ok := ghEvent.(github.IssueCommentEvent); ok {
			prBranchName, _, err := githubPrService.GetBranchName(*commentEvent.Issue.Number)

			if err != nil {
				usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Error while retriving default branch from Issue: %v", err), 6)
			}
			defaultBranch := *commentEvent.Repo.DefaultBranch
			repoFullName := *commentEvent.Repo.FullName
			requestedBy := *commentEvent.Sender.Login
			commentBody := *commentEvent.Comment.Body
			jobs, coversAllImpactedProjects, err = generic.ConvertIssueCommentEventToJobs(repoFullName, requestedBy, prNumber, commentBody, impactedProjects, requestedProject, diggerConfig.Workflows, prBranchName, defaultBranch)
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

		planStorage, err := storage.NewPlanStorage(ghToken, repoOwner, repositoryName, &prNumber)
		if err != nil {
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to get plan storage. %s", err), 4)
		}

		reporter := &reporting.CiReporter{
			CiService:         &githubPrService,
			PrNumber:          prNumber,
			ReportStrategy:    reportingStrategy,
			IsSupportMarkdown: true,
		}

		jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

		allAppliesSuccessful, atLeastOneApply, err := digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, false, "0", currentDir)
		if !allAppliesSuccessful || err != nil {
			// aggregate status checks: failure
			if scheduler.IsPlanJobs(jobs) {
				githubPrService.SetStatus(prNumber, "failure", "digger/plan")
			} else {
				githubPrService.SetStatus(prNumber, "failure", "digger/apply")
			}
			usage.ReportErrorAndExit(githubActor, fmt.Sprintf("Failed to run commands. %s", err), 8)
		}

		if diggerConfig.AutoMerge && allAppliesSuccessful && atLeastOneApply && coversAllImpactedProjects {
			digger.MergePullRequest(&githubPrService, prNumber)
			log.Println("PR merged successfully")
		}

		if allAppliesSuccessful {
			// aggreate status checks: success
			if scheduler.IsPlanJobs(jobs) {
				githubPrService.SetStatus(prNumber, "success", "digger/plan")
			} else {
				githubPrService.SetStatus(prNumber, "success", "digger/apply")
			}
		}

		log.Println("Commands executed successfully")
	}

	usage.ReportErrorAndExit(githubActor, "Digger finished successfully", 0)
}

func logCommands(projectCommands []scheduler.Job) {
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
