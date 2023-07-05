package digger

import (
	"digger/pkg/ci"
	"digger/pkg/core/execution"
	core_locking "digger/pkg/core/locking"
	"digger/pkg/core/models"
	"digger/pkg/core/policy"
	"digger/pkg/core/reporting"
	"digger/pkg/core/runners"
	"digger/pkg/core/storage"
	"digger/pkg/core/terraform"
	"digger/pkg/core/utils"
	"digger/pkg/locking"
	"digger/pkg/usage"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dominikbraun/graph"
)

type CIName string

const (
	None      = CIName("")
	GitHub    = CIName("github")
	GitLab    = CIName("gitlab")
	BitBucket = CIName("bitbucket")
	Azure     = CIName("azure")
)

func (ci CIName) String() string {
	return string(ci)
}

func DetectCI() CIName {

	notEmpty := func(key string) bool {
		return os.Getenv(key) != ""
	}

	if notEmpty("GITHUB_ACTIONS") {
		return GitHub
	}
	if notEmpty("GITLAB_CI") {
		return GitLab
	}
	if notEmpty("BITBUCKET_BUILD_NUMBER") {
		return BitBucket
	}
	if notEmpty("AZURE_CI") {
		return Azure
	}
	return None

}

func RunCommandsPerProject(
	commandsPerProject []models.ProjectCommand,
	dependencyGraph *graph.Graph[string, string],
	projectNamespace string,
	requestedBy string,
	eventName string,
	prNumber int,
	ciService ci.CIService,
	lock core_locking.Lock,
	reporter reporting.Reporter,
	planStorage storage.PlanStorage,
	policyChecker policy.Checker,
	workingDir string,
) (bool, bool, error) {
	appliesPerProject := make(map[string]bool)

	splits := strings.Split(projectNamespace, "/")
	SCMOrganisation := splits[0]
	SCMrepository := splits[1]

	commandsPerProject = SortedCommandsByDependency(commandsPerProject, dependencyGraph)

	for _, projectCommands := range commandsPerProject {
		for _, command := range projectCommands.Commands {
			fmt.Printf("Running '%s' for project '%s'\n", command, projectCommands.ProjectName)

			allowedToPerformCommand, err := policyChecker.Check(ciService, SCMOrganisation, SCMrepository, projectCommands.ProjectName, command, requestedBy)

			if err != nil {
				return false, false, fmt.Errorf("error checking policy: %v", err)
			}

			if !allowedToPerformCommand {
				msg := fmt.Sprintf("User %s is not allowed to perform action: %s. Check your policies", requestedBy, command)
				err := reporter.Report(msg, utils.AsCollapsibleComment("Policy violation"))
				if err != nil {
					log.Printf("Error publishing comment: %v", err)
				}
				log.Println(msg)
				return false, false, errors.New(msg)
			}

			projectLock := &locking.PullRequestLock{
				InternalLock:     lock,
				Reporter:         reporter,
				CIService:        ciService,
				ProjectName:      projectCommands.ProjectName,
				ProjectNamespace: projectNamespace,
				PrNumber:         prNumber,
			}

			var terraformExecutor terraform.TerraformExecutor
			projectPath := path.Join(workingDir, projectCommands.ProjectDir)
			if projectCommands.Terragrunt {
				terraformExecutor = terraform.Terragrunt{WorkingDir: projectPath}
			} else {
				terraformExecutor = terraform.Terraform{WorkingDir: projectPath, Workspace: projectCommands.ProjectWorkspace}
			}

			commandRunner := runners.CommandRunner{}
			diggerExecutor := execution.DiggerExecutor{
				projectNamespace,
				projectCommands.ProjectName,
				projectPath,
				projectCommands.StateEnvVars,
				projectCommands.CommandEnvVars,
				projectCommands.ApplyStage,
				projectCommands.PlanStage,
				commandRunner,
				terraformExecutor,
				reporter,
				projectLock,
				planStorage,
			}

			switch command {
			case "digger plan":
				err := usage.SendUsageRecord(requestedBy, eventName, "plan")
				if err != nil {
					return false, false, fmt.Errorf("failed to send usage report. %v", err)
				}
				err = ciService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/plan")
				if err != nil {
					return false, false, fmt.Errorf("failed to set PR status. %v", err)
				}
				planPerformed, plan, err := diggerExecutor.Plan()
				if err != nil {
					log.Printf("Failed to run digger plan command. %v", err)
					err := ciService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/plan")
					if err != nil {
						return false, false, fmt.Errorf("failed to set PR status. %v", err)
					}
					return false, false, fmt.Errorf("failed to run digger plan command. %v", err)
				} else if planPerformed {
					if plan != "" {
						formatter := utils.GetTerraformOutputAsCollapsibleComment("Plan for <b>" + projectLock.LockId() + "</b>")
						err = reporter.Report(plan, formatter)
						if err != nil {
							log.Printf("Failed to report plan. %v", err)
						}
					}
					err := ciService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/plan")
					if err != nil {
						return false, false, fmt.Errorf("failed to set PR status. %v", err)
					}
				}
			case "digger apply":
				appliesPerProject[projectCommands.ProjectName] = false
				err := usage.SendUsageRecord(requestedBy, eventName, "apply")
				if err != nil {
					return false, false, fmt.Errorf("failed to send usage report. %v", err)
				}
				err = ciService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/apply")
				if err != nil {
					return false, false, fmt.Errorf("failed to set PR status. %v", err)
				}

				isMerged, err := ciService.IsMerged(prNumber)
				if err != nil {
					return false, false, fmt.Errorf("error checking if PR is merged: %v", err)
				}

				// this might go into some sort of "appliability" plugin later
				isMergeable, err := ciService.IsMergeable(prNumber)
				if err != nil {
					return false, false, fmt.Errorf("error validating is PR is mergeable: %v", err)
				}
				fmt.Printf("PR status, mergeable: %v, merged: %v\n", isMergeable, isMerged)
				if !isMergeable && !isMerged {
					comment := "Cannot perform Apply since the PR is not currently mergeable."
					fmt.Println(comment)
					err = reporter.Report(comment, utils.AsCollapsibleComment("Apply error"))
					if err != nil {
						fmt.Printf("error publishing comment: %v\n", err)
					}
					return false, false, nil
				} else {
					applyPerformed, err := diggerExecutor.Apply()
					if err != nil {
						log.Printf("Failed to run digger apply command. %v", err)
						err := ciService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/apply")
						if err != nil {
							return false, false, fmt.Errorf("failed to set PR status. %v", err)
						}
						return false, false, fmt.Errorf("failed to run digger apply command. %v", err)
					} else if applyPerformed {
						err := ciService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/apply")
						if err != nil {
							return false, false, fmt.Errorf("failed to set PR status. %v", err)
						}
						appliesPerProject[projectCommands.ProjectName] = true
					}
				}
			case "digger unlock":
				err := usage.SendUsageRecord(requestedBy, eventName, "unlock")
				if err != nil {
					return false, false, fmt.Errorf("failed to send usage report. %v", err)
				}
				err = diggerExecutor.Unlock()
				if err != nil {
					return false, false, fmt.Errorf("failed to unlock project. %v", err)
				}
			case "digger lock":
				err := usage.SendUsageRecord(requestedBy, eventName, "lock")
				if err != nil {
					return false, false, fmt.Errorf("failed to send usage report. %v", err)
				}
				err = diggerExecutor.Lock()
				if err != nil {
					return false, false, fmt.Errorf("failed to lock project. %v", err)
				}
			}
		}
	}

	allAppliesSuccess := true
	for _, success := range appliesPerProject {
		if !success {
			allAppliesSuccess = false
		}
	}

	atLeastOneApply := len(appliesPerProject) > 0

	return allAppliesSuccess, atLeastOneApply, nil
}

func SortedCommandsByDependency(project []models.ProjectCommand, dependencyGraph *graph.Graph[string, string]) []models.ProjectCommand {
	var sortedCommands []models.ProjectCommand
	sortedGraph, err := graph.StableTopologicalSort(*dependencyGraph, func(s string, s2 string) bool {
		return s < s2
	})
	if err != nil {
		log.Fatalf("failed to sort commands by dependency, %v", err)
	}
	for _, node := range sortedGraph {
		for _, command := range project {
			if command.ProjectName == node {
				sortedCommands = append(sortedCommands, command)
			}
		}
	}
	return sortedCommands
}

func MergePullRequest(ciService ci.CIService, prNumber int) {
	time.Sleep(5 * time.Second)

	// Check if it was manually merged
	isMerged, err := ciService.IsMerged(prNumber)
	if err != nil {
		log.Fatalf("error checking if PR is merged: %v", err)
	}

	if !isMerged {
		combinedStatus, err := ciService.GetCombinedPullRequestStatus(prNumber)

		if err != nil {
			log.Fatalf("failed to get combined status, %v", err)
		}

		if combinedStatus != "success" {
			log.Fatalf("PR is not mergeable. Status: %v", combinedStatus)
		}

		prIsMergeable, err := ciService.IsMergeable(prNumber)

		if err != nil {
			log.Fatalf("failed to check if PR is mergeable, %v", err)
		}

		if !prIsMergeable {
			log.Fatalf("PR is not mergeable")
		}

		err = ciService.MergePullRequest(prNumber)
		if err != nil {
			log.Fatalf("failed to merge PR, %v", err)
		}
	} else {
	   log.Printf("PR is already merged, skipping merge step")
	}
}
