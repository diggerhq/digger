package digger

import (
	"bytes"
	"digger/pkg/ci"
	"digger/pkg/configuration"
	"digger/pkg/locking"
	"digger/pkg/models"
	"digger/pkg/reporting"
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
	"time"
)

func RunCommandsPerProject(commandsPerProject []models.ProjectCommand, projectNamespace string, requestedBy string, eventName string, prNumber int, ciService ci.CIService, lock locking.Lock, planStorage storage.PlanStorage, workingDir string) (bool, bool, error) {
	appliesPerProject := make(map[string]bool)
	for _, projectCommands := range commandsPerProject {
		for _, command := range projectCommands.Commands {
			projectLock := &locking.CiProjectLock{
				InternalLock:     lock,
				CIService:        ciService,
				ProjectName:      projectCommands.ProjectName,
				ProjectNamespace: projectNamespace,
				PrNumber:         prNumber,
			}
			reporter := &reporting.CiReporter{
				CiService: ciService,
				PrNumber:  prNumber,
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

func MergePullRequest(githubPrService ci.CIService, prNumber int) {
	time.Sleep(5 * time.Second)
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

type DiggerExecutor struct {
	ProjectNamespace  string
	ProjectName       string
	ProjectPath       string
	StateEnvVars      map[string]string
	CommandEnvVars    map[string]string
	ApplyStage        *configuration.Stage
	PlanStage         *configuration.Stage
	CommandRunner     CommandRun
	TerraformExecutor terraform.TerraformExecutor
	Reporter          reporting.Reporter
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
	return d.ProjectNamespace + "#" + d.ProjectName + ".tfplan"
}

func (d DiggerExecutor) localPlanFilePath() string {
	return path.Join(d.ProjectPath, d.planFileName())
}

func (d DiggerExecutor) storedPlanFilePath() string {
	return path.Join(d.ProjectNamespace, d.planFileName())
}

func (d DiggerExecutor) Plan() (bool, error) {
	locked, err := d.ProjectLock.Lock()
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
				err = d.Reporter.Report(comment)
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

func (d DiggerExecutor) Apply() (bool, error) {
	var plansFilename *string
	if d.PlanStorage != nil {
		var err error
		plansFilename, err = d.PlanStorage.RetrievePlan(d.localPlanFilePath(), d.storedPlanFilePath())
		if err != nil {
			return false, fmt.Errorf("error retrieving plan: %v", err)
		}
	}

	locked, err := d.ProjectLock.Lock()

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
				commentErr := d.Reporter.Report(comment)
				if commentErr != nil {
					fmt.Printf("error publishing comment: %v", err)
				}
				if err != nil {
					commentErr = d.Reporter.Report("Error during applying.")
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

func (d DiggerExecutor) Unlock() error {
	err := d.ProjectLock.ForceUnlock()
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

func (d DiggerExecutor) Lock() error {
	_, err := d.ProjectLock.Lock()
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
