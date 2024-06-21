package main

import (
	"fmt"
	core_backend "github.com/diggerhq/digger/cli/pkg/core/backend"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	core_locking "github.com/diggerhq/digger/libs/locking"
	orchestrator "github.com/diggerhq/digger/libs/orchestrator"
	"log"
	"os"
)

func exec(actor string, projectName string, repoNamespace string, command string, prNumber int, lock core_locking.Lock, policyChecker core_policy.Checker, prService orchestrator.PullRequestService, orgService orchestrator.OrgService, reporter reporting.Reporter, backendApi core_backend.Api) {

	//SCMOrganisation, SCMrepository := utils.ParseRepoNamespace(runConfig.RepoNamespace)
	currentDir, err := os.Getwd()
	if err != nil {

		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)

	}

	planStorage := storage.NewPlanStorage("", "", "", actor, nil)

	changedFiles, err := prService.GetChangedFiles(prNumber)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("could not get changed files: %v", err), 1)
	}
	diggerConfig, _, dependencyGraph, err := digger_config.LoadDiggerConfig("./", true, changedFiles)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to load digger config. %s", err), 4)
	}
	//impactedProjects := diggerConfig.GetModifiedProjects(strings.Split(runConfig.FilesChanged, ","))
	impactedProjects := diggerConfig.GetProjects(projectName)
	jobs, _, err := orchestrator.ConvertProjectsToJobs(actor, repoNamespace, command, prNumber, impactedProjects, nil, diggerConfig.Workflows)
	if err != nil {
		usage.ReportErrorAndExit(actor, fmt.Sprintf("Failed to convert impacted projects to commands. %s", err), 4)
	}

	jobs = digger.SortedCommandsByDependency(jobs, &dependencyGraph)
	_, _, err = digger.RunJobs(jobs, prService, orgService, lock, reporter, planStorage, policyChecker, comment_updater.NoopCommentUpdater{}, backendApi, "", false, false, 123, currentDir, true)
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
		usage.ReportErrorAndExit("", fmt.Sprintf("Error occured during command exec: %v", err), 8)
	}

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

func init() {
	log.SetOutput(os.Stdout)

	if os.Getenv("DEBUG") == "true" {
		log.SetFlags(log.Ltime | log.Lshortfile)
	}
}
