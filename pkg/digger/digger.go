package digger

import (
	"bytes"
	"digger/pkg/ci"
	"digger/pkg/configuration"
	"digger/pkg/locking"
	"digger/pkg/models"
	"digger/pkg/storage"
	"digger/pkg/terraform"
	"digger/pkg/usage"
	"digger/pkg/utils"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

func RunCommandsPerProject(commandsPerProject []models.ProjectCommand, repoOwner string, repoName string, eventName string, prNumber int, ciService ci.CIService, lock locking.Lock, planStorage storage.PlanStorage, workingDir string) (bool, error) {
	allAppliesSuccess := true
	appliesPerProject := make(map[string]bool)
	for _, projectCommands := range commandsPerProject {
		appliesPerProject[projectCommands.ProjectName] = false
		for _, command := range projectCommands.Commands {
			projectLock := &locking.ProjectLockImpl{
				InternalLock: lock,
				CIService:    ciService,
				ProjectName:  projectCommands.ProjectName,
				RepoName:     repoName,
				RepoOwner:    repoOwner,
			}

			var terraformExecutor terraform.TerraformExecutor
			projectPath := path.Join(workingDir, projectCommands.ProjectDir)
			if projectCommands.Terragrunt {
				terraformExecutor = terraform.Terragrunt{WorkingDir: projectPath}
			} else {
				terraformExecutor = terraform.Terraform{WorkingDir: projectPath, Workspace: projectCommands.ProjectWorkspace}
			}

			commandRunner := CommandRunner{}
			diggerExecutor := DiggerExecutor{
				repoOwner,
				repoName,
				projectCommands.ProjectName,
				projectPath,
				projectCommands.StateEnvVars,
				projectCommands.CommandEnvVars,
				projectCommands.ApplyStage,
				projectCommands.PlanStage,
				commandRunner,
				terraformExecutor,
				ciService,
				projectLock,
				planStorage,
			}
			switch command {
			case "digger plan":
				usage.SendUsageRecord(repoOwner, eventName, "plan")
				ciService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/plan")
				planPerformed, err := diggerExecutor.Plan(prNumber)
				if err != nil {
					log.Printf("Failed to run digger plan command. %v", err)
					ciService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/plan")
					return false, fmt.Errorf("failed to run digger plan command. %v", err)
				} else if planPerformed {
					ciService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/plan")
				}
			case "digger apply":
				usage.SendUsageRecord(repoName, eventName, "apply")
				ciService.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/apply")
				applyPerformed, err := diggerExecutor.Apply(prNumber)
				if err != nil {
					log.Printf("Failed to run digger apply command. %v", err)
					ciService.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/apply")
					return false, fmt.Errorf("failed to run digger apply command. %v", err)
				} else if applyPerformed {
					ciService.SetStatus(prNumber, "success", projectCommands.ProjectName+"/apply")
					appliesPerProject[projectCommands.ProjectName] = true
				}
			case "digger unlock":
				usage.SendUsageRecord(repoOwner, eventName, "unlock")
				err := diggerExecutor.Unlock(prNumber)
				if err != nil {
					return false, fmt.Errorf("failed to unlock project. %v", err)
				}
			case "digger lock":
				usage.SendUsageRecord(repoOwner, eventName, "lock")
				err := diggerExecutor.Lock(prNumber)
				if err != nil {
					return false, fmt.Errorf("failed to lock project. %v", err)
				}
			}
		}
	}

	for _, success := range appliesPerProject {
		if !success {
			allAppliesSuccess = false
		}
	}
	return allAppliesSuccess, nil
}

func MergePullRequest(githubPrService ci.CIService, prNumber int) {
	combinedStatus, err := githubPrService.GetCombinedPullRequestStatus(prNumber)

	if err != nil {
		log.Fatalf("failed to get combined status, %v", err)
	}

	if combinedStatus != "success" {
		log.Fatalf("PR is not mergeable. Status: %v", combinedStatus)
	}

	prIsMergeable, err := githubPrService.IsMergeable(prNumber)

	if err != nil {
		log.Fatalf("failed to check if PR is mergeable, %v", err)
	}

	if !prIsMergeable {
		log.Fatalf("PR is not mergeable")
	}

	err = githubPrService.MergePullRequest(prNumber)
	if err != nil {
		log.Fatalf("failed to merge PR, %v", err)
	}
}

func CollectEnvVars(envs configuration.EnvVars) (map[string]string, map[string]string) {
	stateEnvVars := map[string]string{}

	for _, envvar := range envs.State {
		if envvar.Value != "" {
			stateEnvVars[envvar.Name] = envvar.Value
		} else if envvar.ValueFrom != "" {
			stateEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
		}
	}

	commandEnvVars := map[string]string{}

	for _, envvar := range envs.Commands {
		if envvar.Value != "" {
			commandEnvVars[envvar.Name] = envvar.Value
		} else if envvar.ValueFrom != "" {
			commandEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
		}
	}
	return stateEnvVars, commandEnvVars
}

type DiggerExecutor struct {
	RepoOwner         string
	RepoName          string
	ProjectName       string
	ProjectPath       string
	StateEnvVars      map[string]string
	CommandEnvVars    map[string]string
	ApplyStage        *configuration.Stage
	PlanStage         *configuration.Stage
	CommandRunner     CommandRun
	TerraformExecutor terraform.TerraformExecutor
	CIService         ci.CIService
	ProjectLock       locking.ProjectLock
	PlanStorage       storage.PlanStorage
}

type CommandRun interface {
	Run(workingDir string, shell string, commands []string) (string, string, error)
}

type CommandRunner struct {
}

func (c CommandRunner) Run(workingDir string, shell string, commands []string) (string, string, error) {
	var args []string
	if shell == "" {
		shell = "bash"
		args = []string{"-eo", "pipefail"}
	}

	scriptFile, err := ioutil.TempFile("", "run-script")
	if err != nil {
		return "", "", fmt.Errorf("error creating script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())

	for _, command := range commands {
		_, err := scriptFile.WriteString(command + "\n")
		if err != nil {
			return "", "", fmt.Errorf("error writing to script file: %v", err)
		}
	}
	args = append(args, scriptFile.Name())

	cmd := exec.Command(shell, args...)
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	mwout := io.MultiWriter(os.Stdout, &stdout)
	mwerr := io.MultiWriter(os.Stderr, &stderr)
	cmd.Stdout = mwout
	cmd.Stderr = mwerr
	err = cmd.Run()

	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("error: %v", err)
	}

	return stdout.String(), stderr.String(), err
}

func (d DiggerExecutor) planFileName() string {
	return d.RepoName + "#" + d.ProjectName + ".tfplan"
}

func (d DiggerExecutor) localPlanFilePath() string {
	return path.Join(d.ProjectPath, d.planFileName())
}

func (d DiggerExecutor) storedPlanFilePath() string {
	return path.Join(d.RepoOwner, d.planFileName())
}

func (d DiggerExecutor) Plan(prNumber int) (bool, error) {
	locked, err := d.ProjectLock.Lock(prNumber)
	if err != nil {
		return false, fmt.Errorf("error locking project: %v", err)
	}
	log.Printf("Lock result: %t\n", locked)
	if locked {
		var planSteps []configuration.Step

		if d.PlanStage != nil {
			planSteps = d.PlanStage.Steps
		} else {
			planSteps = []configuration.Step{
				{
					Action: "init",
				},
				{
					Action: "plan",
				},
			}
		}
		for _, step := range planSteps {
			if step.Action == "init" {
				_, _, err := d.TerraformExecutor.Init(step.ExtraArgs, d.StateEnvVars)
				if err != nil {
					return false, fmt.Errorf("error running init: %v", err)
				}
			}
			if step.Action == "plan" {
				planArgs := []string{"-out", d.planFileName()}
				planArgs = append(planArgs, step.ExtraArgs...)
				isNonEmptyPlan, stdout, stderr, err := d.TerraformExecutor.Plan(planArgs, d.CommandEnvVars)
				if err != nil {
					return false, fmt.Errorf("error executing plan: %v", err)
				}
				if d.PlanStorage != nil {
					planExists, err := d.PlanStorage.PlanExists(d.storedPlanFilePath())
					if err != nil {
						return false, fmt.Errorf("error checking if plan exists: %v", err)
					}

					if planExists {
						err = d.PlanStorage.DeleteStoredPlan(d.storedPlanFilePath())
						if err != nil {
							return false, fmt.Errorf("error deleting plan: %v", err)
						}
					}

					err = d.PlanStorage.StorePlan(d.localPlanFilePath(), d.storedPlanFilePath())
					if err != nil {
						return false, fmt.Errorf("error storing plan: %v", err)
					}
				}
				plan := cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
				comment := utils.GetTerraformOutputAsCollapsibleComment("Plan for **"+d.ProjectLock.LockId()+"**", plan)
				err = d.CIService.PublishComment(prNumber, comment)
				if err != nil {
					fmt.Printf("error publishing comment: %v", err)
				}
			}
			if step.Action == "run" {
				var commands []string
				if os.Getenv("ACTIVATE_VENV") == "true" {
					commands = append(commands, fmt.Sprintf("source %v/.venv/bin/activate", os.Getenv("GITHUB_WORKSPACE")))
				}
				commands = append(commands, step.Value)
				log.Printf("Running %v for **%v**\n", step.Value, d.ProjectLock.LockId())
				_, _, err := d.CommandRunner.Run(d.ProjectPath, step.Shell, commands)
				if err != nil {
					return false, fmt.Errorf("error running command: %v", err)
				}
			}
		}
		return true, nil
	}
	return false, nil
}

func (d DiggerExecutor) Apply(prNumber int) (bool, error) {
	var plansFilename *string
	if d.PlanStorage != nil {
		var err error
		plansFilename, err = d.PlanStorage.RetrievePlan(d.localPlanFilePath(), d.storedPlanFilePath())
		if err != nil {
			return false, fmt.Errorf("error retrieving plan: %v", err)
		}
	}

	isMergeable, err := d.CIService.IsMergeable(prNumber)
	if err != nil {
		return false, fmt.Errorf("error validating is PR is mergeable: %v", err)
	}
	if !isMergeable {
		comment := "Cannot perform Apply since the PR is not currently mergeable."
		err = d.CIService.PublishComment(prNumber, comment)
		if err != nil {
			fmt.Printf("error publishing comment: %v", err)
		}
		return false, nil
	} else {
		locked, err := d.ProjectLock.Lock(prNumber)

		if err != nil {
			return false, fmt.Errorf("error locking project: %v", err)
		}

		if locked {
			var applySteps []configuration.Step

			if d.ApplyStage != nil {
				applySteps = d.ApplyStage.Steps
			} else {
				applySteps = []configuration.Step{
					{
						Action: "init",
					},
					{
						Action: "apply",
					},
				}
			}

			for _, step := range applySteps {
				if step.Action == "init" {
					_, _, err := d.TerraformExecutor.Init(step.ExtraArgs, d.StateEnvVars)
					if err != nil {
						return false, fmt.Errorf("error running init: %v", err)
					}
				}
				if step.Action == "apply" {
					stdout, stderr, err := d.TerraformExecutor.Apply(step.ExtraArgs, plansFilename, d.CommandEnvVars)
					applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
					comment := utils.GetTerraformOutputAsCollapsibleComment("Apply for **"+d.ProjectLock.LockId()+"**", applyOutput)
					commentErr := d.CIService.PublishComment(prNumber, comment)
					if commentErr != nil {
						fmt.Printf("error publishing comment: %v", err)
					}
					if err != nil {
						commentErr = d.CIService.PublishComment(prNumber, "Error during applying.")
						if commentErr != nil {
							fmt.Printf("error publishing comment: %v", err)
						}
						return false, fmt.Errorf("error executing apply: %v", err)
					}
				}
				if step.Action == "run" {
					var commands []string
					if os.Getenv("ACTIVATE_VENV") == "true" {
						commands = append(commands, fmt.Sprintf("source %v/.venv/bin/activate", os.Getenv("GITHUB_WORKSPACE")))
					}
					commands = append(commands, step.Value)
					log.Printf("Running %v for **%v**\n", step.Value, d.ProjectLock.LockId())
					_, _, err := d.CommandRunner.Run(d.ProjectPath, step.Shell, commands)
					if err != nil {
						return false, fmt.Errorf("error running command: %v", err)
					}
				}
			}
			return true, nil
		} else {
			return false, nil
		}
	}
}

func (d DiggerExecutor) Unlock(prNumber int) error {
	err := d.ProjectLock.ForceUnlock(prNumber)
	if err != nil {
		return fmt.Errorf("failed to aquire lock: %s, %v", d.ProjectLock.LockId(), err)
	}
	if d.PlanStorage != nil {
		err = d.PlanStorage.DeleteStoredPlan(d.storedPlanFilePath())
		if err != nil {
			return fmt.Errorf("failed to delete stored plan file '%v':  %v", d.storedPlanFilePath(), err)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to delete stored plan file '%v':  %v", d.storedPlanFilePath(), err)
	}
	return nil
}

func (d DiggerExecutor) Lock(prNumber int) error {
	_, err := d.ProjectLock.Lock(prNumber)
	if err != nil {
		return fmt.Errorf("failed to aquire lock: %s, %v", d.ProjectLock.LockId(), err)
	}
	return nil
}

func cleanupTerraformOutput(nonEmptyOutput bool, planError error, stdout string, stderr string, regexStr *string) string {
	var errorStr, start string

	// removes output of terraform -version command that terraform-exec executes on every run
	i := strings.Index(stdout, "Initializing the backend...")
	if i != -1 {
		stdout = stdout[i:]
	}
	endPos := len(stdout)

	if planError != nil {
		if stderr != "" {
			errorStr = stderr
		} else if stdout != "" {
			errorStr = stdout
		}
		return errorStr
	} else if nonEmptyOutput {
		start = "Terraform will perform the following actions:"
	} else {
		start = "No changes. Your infrastructure matches the configuration."
	}

	startPos := strings.Index(stdout, start)
	if startPos == -1 {
		startPos = 0
	}

	if regexStr != nil {
		regex := regexp.MustCompile(*regexStr)
		matches := regex.FindStringSubmatch(stdout)
		if len(matches) > 0 {
			firstMatch := matches[0]
			endPos = strings.LastIndex(stdout, firstMatch) + len(firstMatch)
		}
	}

	return stdout[startPos:endPos]
}

func cleanupTerraformApply(nonEmptyPlan bool, planError error, stdout string, stderr string) string {
	return cleanupTerraformOutput(nonEmptyPlan, planError, stdout, stderr, nil)
}

func cleanupTerraformPlan(nonEmptyPlan bool, planError error, stdout string, stderr string) string {
	regex := `───────────.+`
	return cleanupTerraformOutput(nonEmptyPlan, planError, stdout, stderr, &regex)
}

func DefaultWorkflow() *configuration.Workflow {
	return &configuration.Workflow{
		Configuration: &configuration.WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &configuration.Stage{
			Steps: []configuration.Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &configuration.Stage{
			Steps: []configuration.Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
	}
}

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
