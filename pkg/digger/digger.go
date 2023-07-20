package digger

import (
	"digger/pkg/ci"
	"digger/pkg/core/execution"
	core_locking "digger/pkg/core/locking"
	"digger/pkg/core/models"
	"digger/pkg/core/policy"
	core_reporting "digger/pkg/core/reporting"
	"digger/pkg/core/runners"
	"digger/pkg/core/storage"
	"digger/pkg/core/terraform"
	"digger/pkg/core/utils"
	"digger/pkg/locking"
	"digger/pkg/reporting"
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
	prService ci.PullRequestService,
	orgService ci.OrgService,
	lock core_locking.Lock,
	reporter core_reporting.Reporter,
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

			allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, SCMOrganisation, SCMrepository, projectCommands.ProjectName, command, requestedBy)

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
				CIService:        prService,
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
			planPathProvider := execution.ProjectPathProvider{
				ProjectPath:      projectPath,
				ProjectNamespace: projectNamespace,
				ProjectName:      projectCommands.ProjectName,
			}
			diggerExecutor := execution.LockingExecutorWrapper{
				ProjectLock: projectLock,
				Executor: execution.DiggerExecutor{
					ProjectNamespace:  projectNamespace,
					ProjectName:       projectCommands.ProjectName,
					ProjectPath:       projectPath,
					StateEnvVars:      projectCommands.StateEnvVars,
					CommandEnvVars:    projectCommands.CommandEnvVars,
					ApplyStage:        projectCommands.ApplyStage,
					PlanStage:         projectCommands.PlanStage,
					CommandRunner:     commandRunner,
					TerraformExecutor: terraformExecutor,
					Reporter:          reporter,
					PlanStorage:       planStorage,
					PlanPathProvider:  planPathProvider,
				},
			}

			switch command {
			case "digger plan":
				err := usage.SendUsageRecord(requestedBy, eventName, "plan")
				if err != nil {
					return false, false, fmt.Errorf("failed to send usage report. %v", err)
				}
				err = prService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/plan")
				if err != nil {
					return false, false, fmt.Errorf("failed to set PR status. %v", err)
				}
				planPerformed, plan, planJsonOutput, err := diggerExecutor.Plan()

				if err != nil {
					log.Printf("Failed to run digger plan command. %v", err)
					err := prService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/plan")
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
						planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, projectCommands.ProjectName, planJsonOutput)
						if err != nil {
							log.Printf("failed to validate plan %v", err)
							return false, false, fmt.Errorf("failed to validated plan %v", err)
						}
						planPolicyFormatter := utils.AsCollapsibleComment(fmt.Sprintf("Terraform plan validation check (%v)", projectCommands.ProjectName))
						if !planIsAllowed {
							planReportMessage := "Terraform plan failed validation checks :x:\n"
							planReportMessage = planReportMessage + "    " + strings.Join(messages, "   \n")
							err = reporter.Report(planReportMessage, planPolicyFormatter)

							if err != nil {
								log.Printf("Failed to report plan. %v", err)
							}
							log.Printf("Plan is not allowed")
							return false, false, fmt.Errorf("Plan is not allowed")
						} else {
							reporter.Report("Terraform plan validation checks succeeded :white_check_mark:", planPolicyFormatter)
						}
					}
					err := prService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/plan")
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
				err = prService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/apply")
				if err != nil {
					return false, false, fmt.Errorf("failed to set PR status. %v", err)
				}

				isMerged, err := prService.IsMerged(prNumber)
				if err != nil {
					return false, false, fmt.Errorf("error checking if PR is merged: %v", err)
				}

				// this might go into some sort of "appliability" plugin later
				isMergeable, err := prService.IsMergeable(prNumber)
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
						err := prService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/apply")
						if err != nil {
							return false, false, fmt.Errorf("failed to set PR status. %v", err)
						}
						return false, false, fmt.Errorf("failed to run digger apply command. %v", err)
					} else if applyPerformed {
						err := prService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/apply")
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

				if planStorage != nil {
					err = planStorage.DeleteStoredPlan(planPathProvider.StoredPlanFilePath())
					if err != nil {
						log.Printf("failed to delete stored plan file '%v':  %v", planPathProvider.StoredPlanFilePath(), err)
					}
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

func RunCommandForProject(
	commands models.ProjectCommand,
	projectNamespace string,
	requestedBy string,
	eventName string,
	orgService ci.OrgService,
	policyChecker policy.Checker,
	planStorage storage.PlanStorage,
	workingDir string,
) error {
	splits := strings.Split(projectNamespace, "/")
	SCMOrganisation := splits[0]
	SCMrepository := splits[1]
	fmt.Printf("Running '%s' for project '%s'\n", commands.Commands, commands.ProjectName)

	for _, command := range commands.Commands {

		allowedToPerformCommand, err := policyChecker.CheckAccessPolicy(orgService, SCMOrganisation, SCMrepository, commands.ProjectName, command, requestedBy)

		if err != nil {
			return fmt.Errorf("error checking policy: %v", err)
		}

		if !allowedToPerformCommand {
			msg := fmt.Sprintf("User %s is not allowed to perform action: %s. Check your policies", requestedBy, command)
			if err != nil {
				log.Printf("Error publishing comment: %v", err)
			}
			log.Println(msg)
			return errors.New(msg)
		}
		var terraformExecutor terraform.TerraformExecutor
		projectPath := path.Join(workingDir, commands.ProjectDir)
		if commands.Terragrunt {
			terraformExecutor = terraform.Terragrunt{WorkingDir: projectPath}
		} else {
			terraformExecutor = terraform.Terraform{WorkingDir: projectPath, Workspace: commands.ProjectWorkspace}
		}

		commandRunner := runners.CommandRunner{}

		planPathProvider := execution.ProjectPathProvider{
			ProjectPath:      projectPath,
			ProjectNamespace: projectNamespace,
			ProjectName:      commands.ProjectName,
		}

		diggerExecutor := execution.DiggerExecutor{
			ProjectNamespace:  projectNamespace,
			ProjectName:       commands.ProjectName,
			ProjectPath:       projectPath,
			StateEnvVars:      commands.StateEnvVars,
			CommandEnvVars:    commands.CommandEnvVars,
			ApplyStage:        commands.ApplyStage,
			PlanStage:         commands.PlanStage,
			CommandRunner:     commandRunner,
			Reporter:          &reporting.StdOutReporter{},
			TerraformExecutor: terraformExecutor,
			PlanStorage:       planStorage,
			PlanPathProvider:  planPathProvider,
		}

		switch command {
		case "digger plan":
			err := usage.SendUsageRecord(requestedBy, eventName, "plan")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, _, planJsonOutput, err := diggerExecutor.Plan()
			if err != nil {
				log.Printf("Failed to run digger plan command. %v", err)
				return fmt.Errorf("failed to run digger plan command. %v", err)
			}
			planIsAllowed, messages, err := policyChecker.CheckPlanPolicy(SCMrepository, commands.ProjectName, planJsonOutput)
			fmt.Printf(strings.Join(messages, "\n"))
			if err != nil {
				log.Printf("failed to validate plan %v", err)
				return fmt.Errorf("failed to validated plan %v", err)
			}
			if !planIsAllowed {
				log.Printf("Plan is not allowed")
				return fmt.Errorf("Plan is not allowed")
			}

		case "digger apply":
			err := usage.SendUsageRecord(requestedBy, eventName, "apply")
			if err != nil {
				log.Printf("Failed to send usage report. %v", err)
			}
			_, err = diggerExecutor.Apply()
			if err != nil {
				log.Printf("Failed to run digger apply command. %v", err)
				return fmt.Errorf("failed to run digger apply command. %v", err)
			}
		}
	}
	return nil
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

func MergePullRequest(ciService ci.PullRequestService, prNumber int) {
	time.Sleep(5 * time.Second)

	// CheckAccessPolicy if it was manually merged
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
