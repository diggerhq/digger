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
	"digger/pkg/locking"
	"digger/pkg/usage"
	"fmt"
	"log"
	"os"
	"path"
	"time"
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

type PolicyInput struct {
	user   string
	action string
}

func RunCommandsPerProject(commandsPerProject []models.ProjectCommand, projectNamespace string, requestedBy string, eventName string, prNumber int, ciService ci.CIService, lock core_locking.Lock, reporter reporting.Reporter, planStorage storage.PlanStorage, policyChecker policy.Checker, workingDir string) (bool, bool, error) {
	appliesPerProject := make(map[string]bool)
	for _, projectCommands := range commandsPerProject {
		for _, command := range projectCommands.Commands {

			allowedToPerformCommand, err := policyChecker.Check(projectNamespace, projectCommands.ProjectName, PolicyInput{user: requestedBy, action: command})

			if err != nil {
				return false, false, fmt.Errorf("error checking policy: %v", err)
			}

			if !allowedToPerformCommand {
				return false, false, fmt.Errorf("user %s is not allowed to perform action: %s. Check your policies", requestedBy, command)
			}

			projectLock := &locking.PullRequestLock{
				InternalLock:     lock,
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
				usage.SendUsageRecord(requestedBy, eventName, "plan")
				ciService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/plan")
				planPerformed, err := diggerExecutor.Plan()
				if err != nil {
					log.Printf("Failed to run digger plan command. %v", err)
					ciService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/plan")
					return false, false, fmt.Errorf("failed to run digger plan command. %v", err)
				} else if planPerformed {
					ciService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/plan")
				}
			case "digger apply":
				appliesPerProject[projectCommands.ProjectName] = false
				usage.SendUsageRecord(requestedBy, eventName, "apply")
				ciService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/apply")

				// this might go into some sort of "appliability" plugin later
				isMergeable, err := ciService.IsMergeable(prNumber)
				if err != nil {
					return false, false, fmt.Errorf("error validating is PR is mergeable: %v", err)
				}
				if !isMergeable {
					comment := "Cannot perform Apply since the PR is not currently mergeable."
					err = ciService.PublishComment(prNumber, comment)
					if err != nil {
						fmt.Printf("error publishing comment: %v", err)
					}
					return false, false, nil
				} else {
					applyPerformed, err := diggerExecutor.Apply()
					if err != nil {
						log.Printf("Failed to run digger apply command. %v", err)
						ciService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/apply")
						return false, false, fmt.Errorf("failed to run digger apply command. %v", err)
					} else if applyPerformed {
						ciService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/apply")
						appliesPerProject[projectCommands.ProjectName] = true
					}
				}
			case "digger unlock":
				usage.SendUsageRecord(requestedBy, eventName, "unlock")
				err := diggerExecutor.Unlock()
				if err != nil {
					return false, false, fmt.Errorf("failed to unlock project. %v", err)
				}
			case "digger lock":
				usage.SendUsageRecord(requestedBy, eventName, "lock")
				err := diggerExecutor.Lock()
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

func MergePullRequest(ciService ci.CIService, prNumber int) {
	time.Sleep(5 * time.Second)
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
}
