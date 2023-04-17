package digger

import (
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/terraform"
	"digger/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
)

func ProcessGitHubEvent(ghEvent models.Event, diggerConfig *DiggerConfig, prManager github.PullRequestManager) ([]Project, int, error) {
	var impactedProjects []Project
	var prNumber int

	switch ghEvent.(type) {
	case models.PullRequestEvent:
		prNumber = ghEvent.(models.PullRequestEvent).PullRequest.Number
		changedFiles, err := prManager.GetChangedFiles(prNumber)

		if err != nil {
			return nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case models.IssueCommentEvent:
		prNumber = ghEvent.(models.IssueCommentEvent).Issue.Number
		requestedProject := parseProjectName(ghEvent.(models.IssueCommentEvent).Comment.Body)
		if requestedProject != "" {
			impactedProjects = diggerConfig.GetProjects(requestedProject)
		} else {
			changedFiles, err := prManager.GetChangedFiles(prNumber)
			if err != nil {
				log.Fatalf("Could not get changed files")
			}
			impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		}
	default:
		return nil, 0, fmt.Errorf("unsupported event type")
	}
	return impactedProjects, prNumber, nil
}

func RunCommandsPerProject(commandsPerProject []ProjectCommand, repoOwner string, repoName string, eventName string, prNumber int, diggerConfig *DiggerConfig, prManager github.PullRequestManager, lock utils.Lock, workingDir string) (bool, error) {
	lockAcquisitionSuccess := true
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
			diggerExecutor := DiggerExecutor{
				workingDir,
				projectCommands.ProjectWorkspace,
				repoOwner,
				projectCommands.ProjectName,
				projectCommands.ProjectDir,
				repoName,
				projectCommands.Terragrunt,
				projectCommands.ApplyStage,
				projectCommands.PlanStage,
				prManager,
				projectLock,
				diggerConfig,
			}
			switch command {
			case "digger plan":
				utils.SendUsageRecord(repoOwner, eventName, "plan")
				prManager.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/plan")
				err := diggerExecutor.Plan(prNumber)
				if err != nil {
					prManager.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/plan")
				} else {
					prManager.SetStatus(prNumber, "success", projectCommands.ProjectName+"/plan")
				}
			case "digger apply":
				utils.SendUsageRecord(repoName, eventName, "apply")
				prManager.SetStatus(prNumber, "pending", projectCommands.ProjectName+"/apply")
				err := diggerExecutor.Apply(prNumber)
				if err != nil {
					prManager.SetStatus(prNumber, "failure", projectCommands.ProjectName+"/apply")
				} else {
					prManager.SetStatus(prNumber, "success", projectCommands.ProjectName+"/apply")
					appliesPerProject[projectCommands.ProjectName] = true
				}
			case "digger unlock":
				utils.SendUsageRecord(repoOwner, eventName, "unlock")
				diggerExecutor.Unlock(prNumber)
			case "digger lock":
				utils.SendUsageRecord(repoOwner, eventName, "lock")
				lockAcquisitionSuccess = diggerExecutor.Lock(prNumber)
			}
		}
	}

	if !lockAcquisitionSuccess {
		os.Exit(1)
	}
	for _, success := range appliesPerProject {
		if !success {
			allAppliesSuccess = false
		}
	}
	return allAppliesSuccess, nil
}

func MergePullRequest(githubPrService github.PullRequestManager, prNumber int) {
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
	ApplyStage       Stage
	PlanStage        Stage
}

func ConvertGithubEventToCommands(event models.Event, impactedProjects []Project, workflows map[string]Workflow) ([]ProjectCommand, error) {
	commandsPerProject := make([]ProjectCommand, 0)

	switch event.(type) {
	case models.PullRequestEvent:
		event := event.(models.PullRequestEvent)
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = *defaultWorkflow()
			}
			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnCommitToDefault,
					ApplyStage:       *workflow.Apply,
					PlanStage:        *workflow.Plan,
				})
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnPullRequestPushed,
					ApplyStage:       *workflow.Apply,
					PlanStage:        *workflow.Plan,
				})
			} else if event.Action == "closed" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnPullRequestPushed,
					ApplyStage:       *workflow.Apply,
					PlanStage:        *workflow.Plan,
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
						ApplyStage:       *workflow.Apply,
						PlanStage:        *workflow.Plan,
					})
				}
			}
		}
		return commandsPerProject, nil
	default:
		return []ProjectCommand{}, fmt.Errorf("unsupported event type: %T", event)
	}
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
	re := regexp.MustCompile(`-p ([a-zA-Z\-]+)`)
	match := re.FindStringSubmatch(comment)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

type DiggerExecutor struct {
	workingDir   string
	workspace    string
	repoOwner    string
	projectName  string
	projectDir   string
	repoName     string
	terragrunt   bool
	applyStage   Stage
	planStage    Stage
	prManager    github.PullRequestManager
	lock         utils.ProjectLock
	configDigger *DiggerConfig
}

func (d DiggerExecutor) LockId() string {
	return d.repoOwner + "/" + d.repoName + "#" + d.projectName
}

func (d DiggerExecutor) Plan(prNumber int) error {

	var terraformExecutor terraform.TerraformExecutor

	if d.terragrunt {
		terraformExecutor = terraform.Terragrunt{WorkingDir: path.Join(d.workingDir, d.projectDir)}
	} else {
		terraformExecutor = terraform.Terraform{WorkingDir: path.Join(d.workingDir, d.projectDir), Workspace: d.workspace}
	}

	res, err := d.lock.Lock(d.LockId(), prNumber)
	if err != nil {
		return fmt.Errorf("Error locking project: %v", err)
	}
	if res {
		var initArgs []string
		var planArgs []string

		for _, step := range d.planStage.Steps {
			if step.Action == "init" {
				initArgs = append(initArgs, step.ExtraArgs...)
			}
			if step.Action == "plan" {
				planArgs = append(planArgs, step.ExtraArgs...)
			}
		}
		isNonEmptyPlan, stdout, stderr, err := terraformExecutor.Plan(initArgs, planArgs)

		if err != nil {
			return fmt.Errorf("error executing plan: %v", err)
		}
		plan := cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
		comment := "Plan for **" + d.LockId() + "**\n" + plan
		d.prManager.PublishComment(prNumber, comment)
	}
	return nil
}

func (d DiggerExecutor) Apply(prNumber int) error {
	isMergeable, _, err := d.prManager.IsMergeable(prNumber)
	if err != nil {
		return fmt.Errorf("error validating is PR is mergeable: %v", err)
	}

	if !isMergeable {
		comment := "Cannot perform Apply since the PR is not currently mergeable."
		d.prManager.PublishComment(prNumber, comment)
	} else {
		var terraformExecutor terraform.TerraformExecutor

		if d.terragrunt {
			terraformExecutor = terraform.Terragrunt{WorkingDir: path.Join(d.workingDir, d.projectDir)}
		} else {
			terraformExecutor = terraform.Terraform{WorkingDir: path.Join(d.workingDir, d.projectDir), Workspace: d.workspace}
		}

		if res, _ := d.lock.Lock(d.LockId(), prNumber); res {
			var initArgs []string
			var applyArgs []string

			for _, step := range d.applyStage.Steps {
				if step.Action == "init" {
					initArgs = append(initArgs, step.ExtraArgs...)
				}
				if step.Action == "apply" {
					applyArgs = append(applyArgs, step.ExtraArgs...)
				}
			}
			stdout, stderr, err := terraformExecutor.Apply(initArgs, applyArgs)
			applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
			comment := "Apply for **" + d.LockId() + "**\n" + applyOutput
			d.prManager.PublishComment(prNumber, comment)
			if err == nil {
				_, err := d.lock.Unlock(d.LockId(), prNumber)
				if err != nil {
					return fmt.Errorf("error unlocking project: %v", err)
				}
			} else {
				d.prManager.PublishComment(prNumber, "Error during applying. Project lock will persist")
			}
		}
	}
	return nil
}

func (d DiggerExecutor) Unlock(prNumber int) {
	err := d.lock.ForceUnlock(d.LockId(), prNumber)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func (d DiggerExecutor) Lock(prNumber int) bool {
	isLocked, err := d.lock.Lock(d.LockId(), prNumber)
	if err != nil {
		log.Fatalf("Failed to aquire lock: " + d.LockId())
	}
	return isLocked
}

func cleanupTerraformOutput(nonEmptyOutput bool, planError error, stdout string, stderr string, regexStr string) string {
	var errorStr, result, start string

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
		return "```terraform\n" + errorStr + "\n```"
	} else if nonEmptyOutput {
		start = "Terraform will perform the following actions:"
	} else {
		start = "No changes. Your infrastructure matches the configuration."
	}

	startPos := strings.Index(stdout, start)
	if startPos == -1 {
		startPos = 0
	}

	regex := regexp.MustCompile(regexStr)
	matches := regex.FindStringSubmatch(stdout)
	if len(matches) > 0 {
		endPos = strings.Index(stdout, matches[0]) + len(matches[0])
	}

	result = stdout[startPos:endPos]
	return "```terraform\n" + result + "\n```"
}

func cleanupTerraformApply(nonEmptyPlan bool, planError error, stdout string, stderr string) string {
	regex := `(Apply complete! Resources: [0-9]+ added, [0-9]+ changed, [0-9]+ destroyed.)`
	return cleanupTerraformOutput(nonEmptyPlan, planError, stdout, stderr, regex)
}

func cleanupTerraformPlan(nonEmptyPlan bool, planError error, stdout string, stderr string) string {
	regex := `(Plan: [0-9]+ to add, [0-9]+ to change, [0-9]+ to destroy.)`
	return cleanupTerraformOutput(nonEmptyPlan, planError, stdout, stderr, regex)
}

func CheckIfHelpComment(event models.Event) bool {
	switch event.(type) {
	case models.IssueCommentEvent:
		event := event.(models.IssueCommentEvent)
		if strings.Contains(event.Comment.Body, "digger help") {
			return true
		}
	}
	return false
}

func defaultWorkflow() *Workflow {
	return &Workflow{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &Stage{
			Steps: []Step{
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
