package digger

import (
	"bytes"
	"digger/pkg/ci"
	"digger/pkg/configuration"
	"digger/pkg/models"
	"digger/pkg/terraform"
	"digger/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

func ProcessGitHubEvent(ghEvent models.Event, diggerConfig *configuration.DiggerConfig, prManager ci.CIService) ([]configuration.Project, int, bool, error) {
	var impactedProjects []configuration.Project
	var prNumber int
	var mergePrIfCmdSuccessfull = false

	switch ghEvent.(type) {
	case models.PullRequestEvent:
		prNumber = ghEvent.(models.PullRequestEvent).PullRequest.Number
		changedFiles, err := prManager.GetChangedFiles(prNumber)

		if err != nil {
			return nil, 0, false, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case models.IssueCommentEvent:
		prNumber = ghEvent.(models.IssueCommentEvent).Issue.Number
		requestedProject := parseProjectName(ghEvent.(models.IssueCommentEvent).Comment.Body)
		if requestedProject != "" {
			newImpactedProjects := diggerConfig.GetProjects(requestedProject)
			if len(newImpactedProjects) == 0 {
				prManager.PublishComment(prNumber, "Error: Invalid project name '"+requestedProject+"'. The requested operation cannot be performed.")
			} else if len(impactedProjects) == 1 && CheckIfApplyComment(ghEvent) {
				mergePrIfCmdSuccessfull = true
			}
			impactedProjects = newImpactedProjects
		} else {
			mergePrIfCmdSuccessfull = true
			changedFiles, err := prManager.GetChangedFiles(prNumber)
			if err != nil {
				log.Fatalf("Could not get changed files")
			}
			impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		}
	default:
		return nil, 0, false, fmt.Errorf("unsupported event type")
	}
	return impactedProjects, prNumber, mergePrIfCmdSuccessfull, nil
}

func RunCommandsPerProject(commandsPerProject []ProjectCommand, repoOwner string, repoName string, eventName string, prNumber int, prManager ci.CIService, lock utils.Lock, planStorage utils.PlanStorage, workingDir string) (bool, error) {
	allAppliesSuccess := true
	appliesPerProject := make(map[string]bool)
	for _, projectCommands := range commandsPerProject {
		appliesPerProject[projectCommands.ProjectName] = false
		for _, command := range projectCommands.Commands {
			projectLock := &utils.ProjectLockImpl{
				InternalLock: lock,
				PrManager:    prManager,
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
				prManager,
				projectLock,
				planStorage,
			}
			switch command {
			case "digger plan":
				utils.SendUsageRecord(repoOwner, eventName, "plan")
				prManager.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/plan")
				err := diggerExecutor.Plan(prNumber)
				if err != nil {
					log.Printf("Failed to run digger plan command. %v", err)
					prManager.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/plan")

					return false, fmt.Errorf("failed to run digger plan command. %v", err)
				} else {
					prManager.SetStatus(prNumber, "success", projectCommands.ProjectName+"/plan")
				}
			case "digger apply":
				utils.SendUsageRecord(repoName, eventName, "apply")
				prManager.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/apply")
				err := diggerExecutor.Apply(prNumber)
				if err != nil {
					log.Printf("Failed to run digger apply command. %v", err)
					prManager.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/apply")

					return false, fmt.Errorf("failed to run digger apply command. %v", err)
				} else {
					prManager.SetStatus(prNumber, "success", projectCommands.ProjectName+"/apply")
					appliesPerProject[projectCommands.ProjectName] = true
				}
			case "digger unlock":
				utils.SendUsageRecord(repoOwner, eventName, "unlock")
				err := diggerExecutor.Unlock(prNumber)
				if err != nil {
					return false, fmt.Errorf("failed to unlock project. %v", err)
				}
			case "digger lock":
				utils.SendUsageRecord(repoOwner, eventName, "lock")
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

	prIsMergeable, mergeableState, err := githubPrService.IsMergeable(prNumber)

	if err != nil {
		log.Fatalf("failed to check if PR is mergeable, %v", err)
	}

	if !prIsMergeable {
		log.Fatalf("PR is not mergeable. State: %v", mergeableState)
	}

	err = githubPrService.MergePullRequest(prNumber)
	if err != nil {
		log.Fatalf("failed to merge PR, %v", err)
	}
}

func GetGitHubContext(ghContext string) (*models.Github, error) {
	parsedGhContext := new(models.Github)
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return &models.Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

type ProjectCommand struct {
	ProjectName      string
	ProjectDir       string
	ProjectWorkspace string
	Terragrunt       bool
	Commands         []string
	ApplyStage       *configuration.Stage
	PlanStage        *configuration.Stage
	StateEnvVars     map[string]string
	CommandEnvVars   map[string]string
}

func ConvertGithubEventToCommands(event models.Event, impactedProjects []configuration.Project, workflows map[string]configuration.Workflow) ([]ProjectCommand, error) {
	commandsPerProject := make([]ProjectCommand, 0)

	switch event.(type) {
	case models.PullRequestEvent:
		event := event.(models.PullRequestEvent)
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = *defaultWorkflow()
			}

			stateEnvVars, commandEnvVars := collectEnvVars(workflow.EnvVars)

			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnCommitToDefault,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
				})
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnPullRequestPushed,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
				})
			} else if event.Action == "closed" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnPullRequestClosed,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
					CommandEnvVars:   commandEnvVars,
					StateEnvVars:     stateEnvVars,
				})
			}
		}
		return commandsPerProject, nil
	case models.IssueCommentEvent:
		event := event.(models.IssueCommentEvent)
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

		for _, command := range supportedCommands {
			if strings.Contains(event.Comment.Body, command) {
				for _, project := range impactedProjects {
					workflow, ok := workflows[project.Workflow]
					if !ok {
						workflow = *defaultWorkflow()
					}

					stateEnvVars, commandEnvVars := collectEnvVars(workflow.EnvVars)

					workspace := project.Workspace
					workspaceOverride, err := parseWorkspace(event.Comment.Body)
					if err != nil {
						return []ProjectCommand{}, err
					}
					if workspaceOverride != "" {
						workspace = workspaceOverride
					}
					commandsPerProject = append(commandsPerProject, ProjectCommand{
						ProjectName:      project.Name,
						ProjectDir:       project.Dir,
						ProjectWorkspace: workspace,
						Terragrunt:       project.Terragrunt,
						Commands:         []string{command},
						ApplyStage:       workflow.Apply,
						PlanStage:        workflow.Plan,
						CommandEnvVars:   commandEnvVars,
						StateEnvVars:     stateEnvVars,
					})
				}
			}
		}
		return commandsPerProject, nil
	default:
		return []ProjectCommand{}, fmt.Errorf("unsupported event type: %T", event)
	}
}

func collectEnvVars(envs configuration.EnvVars) (map[string]string, map[string]string) {
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

func parseWorkspace(comment string) (string, error) {
	re := regexp.MustCompile(`-w(?:\s+(\S+)|$)`)
	matches := re.FindAllStringSubmatch(comment, -1)

	if len(matches) == 0 {
		return "", nil
	}

	if len(matches) > 1 {
		return "", errors.New("more than one -w flag found")
	}

	if len(matches[0]) < 2 || matches[0][1] == "" {
		return "", errors.New("no value found after -w flag")
	}

	return matches[0][1], nil
}

func parseProjectName(comment string) string {
	re := regexp.MustCompile(`-p ([0-9a-zA-Z\-_]+)`)
	match := re.FindStringSubmatch(comment)
	if len(match) > 1 {
		return match[1]
	}
	return ""
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
	ciService         ci.CIService
	ProjectLock       utils.ProjectLock
	PlanStorage       utils.PlanStorage
}

type CommandRun interface {
	Run(command string) (string, string, error)
}

type CommandRunner struct {
}

func (c CommandRunner) Run(command string) (string, string, error) {
	parts := strings.Fields(command)
	command = parts[0]
	params := parts[1:]
	cmd := exec.Command(command, params...)

	var stdout, stderr bytes.Buffer
	mwout := io.MultiWriter(os.Stdout, &stdout)
	mwerr := io.MultiWriter(os.Stderr, &stderr)
	cmd.Stdout = mwout
	cmd.Stderr = mwerr
	err := cmd.Run()

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

func (d DiggerExecutor) Plan(prNumber int) error {
	res, err := d.ProjectLock.Lock(prNumber)
	if err != nil {
		return fmt.Errorf("error locking project: %v", err)
	}
	log.Printf("Lock result: %t\n", res)
	if res {
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
					return fmt.Errorf("error running init: %v", err)
				}
			}
			if step.Action == "plan" {
				planArgs := []string{"-out", d.planFileName()}
				planArgs = append(planArgs, step.ExtraArgs...)
				isNonEmptyPlan, stdout, stderr, err := d.TerraformExecutor.Plan(planArgs, d.CommandEnvVars)
				if err != nil {
					return fmt.Errorf("error executing plan: %v", err)
				}
				if d.PlanStorage != nil {
					err = d.PlanStorage.StorePlan(d.localPlanFilePath(), d.storedPlanFilePath())
					if err != nil {
						return fmt.Errorf("error storing plan: %v", err)
					}
				}
				plan := cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
				comment := utils.GetTerraformOutputAsCollapsibleComment("Plan for **"+d.ProjectLock.LockId()+"**", plan)
				d.ciService.PublishComment(prNumber, comment)
			}
			if step.Action == "run" {
				stdout, stderr, err := d.CommandRunner.Run(step.Value)
				log.Printf("Running %v for **%v**\n%v%v", step.Value, d.ProjectLock.LockId(), stdout, stderr)
				if err != nil {
					return fmt.Errorf("error running command: %v", err)
				}
			}
		}
	}
	return nil
}

func (d DiggerExecutor) Apply(prNumber int) error {
	var plansFilename *string
	if d.PlanStorage != nil {
		var err error
		plansFilename, err = d.PlanStorage.RetrievePlan(d.localPlanFilePath(), d.storedPlanFilePath())
		if err != nil {
			return fmt.Errorf("error retrieving plan: %v", err)
		}
	}

	isMergeable, _, err := d.ciService.IsMergeable(prNumber)
	if err != nil {
		return fmt.Errorf("error validating is PR is mergeable: %v", err)
	}

	if !isMergeable {
		comment := "Cannot perform Apply since the PR is not currently mergeable."
		d.ciService.PublishComment(prNumber, comment)
	} else {

		if res, _ := d.ProjectLock.Lock(prNumber); res {
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
						return fmt.Errorf("error running init: %v", err)
					}
				}
				if step.Action == "apply" {
					stdout, stderr, err := d.TerraformExecutor.Apply(step.ExtraArgs, plansFilename, d.CommandEnvVars)
					applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
					comment := utils.GetTerraformOutputAsCollapsibleComment("Apply for **"+d.ProjectLock.LockId()+"**", applyOutput)
					d.ciService.PublishComment(prNumber, comment)
					if err != nil {
						d.ciService.PublishComment(prNumber, "Error during applying.")
						return fmt.Errorf("error executing apply: %v", err)
					}
				}
				if step.Action == "run" {
					stdout, stderr, err := d.CommandRunner.Run(step.Value)
					log.Printf("Running %v for **%v**\n%v%v", step.Value, d.ProjectLock.LockId(), stdout, stderr)
					if err != nil {
						return fmt.Errorf("error running command: %v", err)
					}
				}
			}
		}
	}
	return nil
}

func (d DiggerExecutor) Unlock(prNumber int) error {
	err := d.ProjectLock.ForceUnlock(prNumber)
	if err != nil {
		return fmt.Errorf("failed to aquire lock: %s, %v", d.ProjectLock.LockId(), err)
	}
	if d.PlanStage != nil {
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
		if stdout != "" {
			errorStr = stdout
		} else if stderr != "" {
			errorStr = stderr
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
			endPos = strings.Index(stdout, matches[0]) + len(matches[0])
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

func issueCommentEventContainsComment(event models.Event, comment string) bool {
	switch event.(type) {
	case models.IssueCommentEvent:
		event := event.(models.IssueCommentEvent)
		if strings.Contains(event.Comment.Body, comment) {
			return true
		}
	}
	return false
}

func CheckIfHelpComment(event models.Event) bool {
	return issueCommentEventContainsComment(event, "digger help")
}

func CheckIfApplyComment(event models.Event) bool {
	return issueCommentEventContainsComment(event, "digger apply")
}

func defaultWorkflow() *configuration.Workflow {
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
