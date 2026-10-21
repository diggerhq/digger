package forgejo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/cli/pkg/utils"
	core_backend "github.com/diggerhq/digger/libs/backendapi"
	dg_forgejo "github.com/diggerhq/digger/libs/ci/forgejo"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	core_locking "github.com/diggerhq/digger/libs/locking"
	core_policy "github.com/diggerhq/digger/libs/policy"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/storage"
	"gopkg.in/yaml.v3"
)

func ForgejoCI(lock core_locking.Lock, policyCheckerProvider core_policy.PolicyCheckerProvider, backendApi core_backend.Api, reportingStrategy reporting.ReportStrategy, serviceProvider dg_forgejo.ForgejoServiceProvider) {
	slog.Info("Using Forgejo (backendless)")
	actor := os.Getenv("GITHUB_ACTOR")
	usage.SendUsageRecord(actor, "log", "initialize")

	token := os.Getenv("GITHUB_TOKEN")
	if diggerForgejoToken := os.Getenv("DIGGER_FORGEJO_TOKEN"); diggerForgejoToken != "" {
		slog.Info("GITHUB_TOKEN has been overridden with DIGGER_FORGEJO_TOKEN")
		token = diggerForgejoToken
	}
	if token == "" {
		usage.ReportErrorAndExit(actor, "GITHUB_TOKEN is not defined", 1)
	}
	baseUrl := os.Getenv("GITHUB_SERVER_URL")
	if baseUrl == "" {
		usage.ReportErrorAndExit(actor, "GITHUB_SERVER_URL is not defined", 1)
	}
	eventName := os.Getenv("GITHUB_EVENT_NAME")
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventName == "" || eventPath == "" {
		usage.ReportErrorAndExit(actor, "GITHUB_EVENT_NAME or GITHUB_EVENT_PATH is not defined", 2)
	}
	repository := os.Getenv("GITHUB_REPOSITORY")
	if repository == "" {
		usage.ReportErrorAndExit(actor, "GITHUB_REPOSITORY is not defined", 3)
	}

	if os.Getenv("DIGGER_OUT") == "" {
		os.Setenv("DIGGER_OUT", os.Getenv("RUNNER_TEMP")+"/digger-out.log")
	}

	payload, err := os.ReadFile(eventPath)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to read event payload %s. %s", eventPath, err), 2)
	}
	event, err := dg_forgejo.ParseEvent(eventName, payload)
	if errors.Is(err, dg_forgejo.ErrIgnoredEvent) {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Nothing to do. %s", err), 0)
	}
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to parse Forgejo event. %s", err), 6)
	}

	repoOwner, repositoryName := utils.ParseRepoNamespace(repository)
	prService, err := serviceProvider.NewService(baseUrl, token, repositoryName, repoOwner)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("could not create pr service: %v", err), 4)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	diggerConfig, diggerConfigYaml, dependencyGraph, _, err := digger_config.LoadDiggerConfig("./", true, nil, nil)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}

	hostName := os.Getenv("DIGGER_HOSTNAME")
	diggerToken := os.Getenv("DIGGER_TOKEN")
	orgName := os.Getenv("DIGGER_ORGANISATION")
	var policyChecker core_policy.Checker
	if provider, ok := policyCheckerProvider.(core_policy.GitTimeoutPolicyCheckerProvider); ok {
		policyChecker, err = provider.GetWithGitTimeout(hostName, orgName, diggerToken, time.Duration(diggerConfig.GitTimeout)*time.Second)
	} else {
		policyChecker, err = policyCheckerProvider.Get(hostName, orgName, diggerToken)
	}
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to initialize policy checker. %s", err), 4)
	}

	if !diggerConfig.PrLocks {
		slog.Info("Using noop lock as configured in digger.yml")
		lock = core_locking.NoOpLock{}
	}

	yamlData, err := yaml.Marshal(diggerConfigYaml)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("error while marshalling yaml: %s", err), 1)
	}
	for _, p := range diggerConfig.Projects {
		err = backendApi.ReportProject(strings.ReplaceAll(repository, "/", "-"), p.Name, string(yamlData))
		if err != nil {
			slog.Error("Failed to report project", "projectName", p.Name, "error", err)
		}
	}

	impactedProjects, requestedProject, prNumber, err := dg_forgejo.ProcessEvent(event, diggerConfig, &prService)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to process Forgejo event. %s", err), 6)
	}
	for _, p := range impactedProjects {
		slog.Info("project impacted by pull request", "prNumber", prNumber, "project", p.Name)
	}

	if dg_forgejo.IsHelpComment(event) {
		_, err := prService.PublishComment(prNumber, utils.GetCommands())
		if err != nil {
			usage.ReportErrorAndExit(actor, "Failed to publish help command output", 1)
		}
		usage.ReportErrorAndExit(actor, "Published help output", 0)
	}

	if len(impactedProjects) == 0 {
		usage.ReportErrorAndExit(actor, "No projects impacted", 0)
	}

	var jobs []scheduler.Job
	coversAllImpactedProjects := false
	switch e := event.(type) {
	case dg_forgejo.PullRequestEvent:
		coversAllImpactedProjects = true
		jobs, err = dg_forgejo.ConvertPullRequestEventToJobs(e, impactedProjects, *diggerConfig)
	case dg_forgejo.IssueCommentEvent:
		prBranchName, _, _, _, branchErr := prService.GetBranchName(prNumber)
		if branchErr != nil {
			usage.ReportErrorAndExit(actor, fmt.Sprintf("Error while retrieving branch for pull request: %v", branchErr), 6)
		}
		impactedProjectsForEvent := impactedProjects
		if requestedProject != nil {
			impactedProjectsForEvent = []digger_config.Project{*requestedProject}
		}
		jobs, coversAllImpactedProjects, err = generic.ConvertIssueCommentEventToJobs(e.Repository.FullName, e.Sender.UserName, prNumber, e.Comment.Body, impactedProjectsForEvent, impactedProjects, diggerConfig.Workflows, prBranchName, e.Repository.DefaultBranch, true)
	}
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to convert Forgejo event to commands. %s", err), 7)
	}
	if len(jobs) == 0 {
		usage.ReportErrorAndExit(actor, "No commands to run for this event", 0)
	}
	for _, job := range jobs {
		slog.Info("commands to execute", "project", job.ProjectName, "commands", job.Commands)
	}

	err = prService.SetOutput(prNumber, "DIGGER_PR_NUMBER", fmt.Sprintf("%v", prNumber))
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to set job output. Exiting. %s", err), 4)
	}

	planStorage, err := storage.NewPlanStorage(token, repoOwner, repositoryName, &prNumber)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to get plan storage. %s", err), 4)
	}

	reporter := &reporting.CiReporter{
		CiService:         &prService,
		PrNumber:          prNumber,
		ReportStrategy:    reportingStrategy,
		IsSupportMarkdown: true,
	}

	jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)

	statusContext := "digger/apply"
	if scheduler.IsPlanJobs(jobs) {
		statusContext = "digger/plan"
	}
	allAppliesSuccessful, atLeastOneApply, err := digger.RunJobs(jobs, &prService, &prService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, false, "0", currentDir)
	if !allAppliesSuccessful || err != nil {
		prService.SetStatus(prNumber, "failure", statusContext)
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to run commands. %s", err), 8)
	}

	if diggerConfig.AutoMerge && atLeastOneApply && coversAllImpactedProjects {
		digger.MergePullRequest(&prService, prNumber, diggerConfig.AutoMergeStrategy)
		slog.Info("PR merged successfully")
	}

	prService.SetStatus(prNumber, "success", statusContext)
	usage.ReportErrorAndExit(actor, "Digger finished successfully", 0)
}
