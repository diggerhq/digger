package digger

import (
	"digger/pkg/bitbucket"
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/terraform"
	"digger/pkg/utils"
	"encoding/json"
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

func ProcessBitbucketEvent(event bitbucket.Event, diggerConfig *DiggerConfig, prManager github.PullRequestManager) ([]Project, int, error) {
	var impactedProjects []Project
	var prNumber int

	switch event.Type {
	case PullRequestCreated:
		prNumber = ghEvent.(models.PullRequestEvent).PullRequest.Number
		changedFiles, err := prManager.GetChangedFiles(prNumber)

		if err != nil {
			return nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case PullRequestUpdated:
		prNumber = ghEvent.(models.PullRequestEvent).PullRequest.Number
		changedFiles, err := prManager.GetChangedFiles(prNumber)

		if err != nil {
			return nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case PullRequestClosed:
		prNumber = ghEvent.(models.PullRequestEvent).PullRequest.Number
		changedFiles, err := prManager.GetChangedFiles(prNumber)

		if err != nil {
			return nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case PullRequestApproved:
	case CommentCreated:

	default:
		return nil, 0, fmt.Errorf("unsupported bitbucket event type")
	}
	return impactedProjects, prNumber, nil
}

func RunCommandsPerProject(commandsPerProject []ProjectCommand, repoOwner string, repoName string, eventName string, prNumber int, diggerConfig *DiggerConfig, prManager github.PullRequestManager, lock utils.Lock, workingDir string) error {
	lockAcquisitionSuccess := true
	for _, projectCommands := range commandsPerProject {
		for _, command := range projectCommands.Commands {
			projectLock := &utils.ProjectLockImpl{
				InternalLock: lock,
				PrManager:    prManager,
				ProjectName:  projectCommands.ProjectName,
				RepoName:     repoName,
			}
			diggerExecutor := DiggerExecutor{
				workingDir,
				repoOwner,
				projectCommands.ProjectName,
				projectCommands.ProjectDir,
				repoName,
				prManager,
				projectLock,
				diggerConfig,
			}
			switch command {
			case "digger plan":
				utils.SendUsageRecord(repoOwner, eventName, "plan")
				diggerExecutor.Plan(prNumber)
			case "digger apply":
				utils.SendUsageRecord(repoName, eventName, "apply")
				diggerExecutor.Apply(prNumber)
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
	return nil
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
	ProjectName string
	ProjectDir  string
	Commands    []string
}

func ConvertGithubEventToCommands(event models.Event, impactedProjects []Project) ([]ProjectCommand, error) {
	commandsPerProject := make([]ProjectCommand, 0)

	switch event.(type) {
	case models.PullRequestEvent:
		event := event.(models.PullRequestEvent)
		for _, project := range impactedProjects {
			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName: project.Name,
					ProjectDir:  project.Dir,
					Commands:    project.WorkflowConfiguration.OnCommitToDefault,
				})
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName: project.Name,
					ProjectDir:  project.Dir,
					Commands:    project.WorkflowConfiguration.OnPullRequestPushed,
				})
			} else if event.Action == "closed" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName: project.Name,
					ProjectDir:  project.Dir,
					Commands:    project.WorkflowConfiguration.OnPullRequestClosed,
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
					commandsPerProject = append(commandsPerProject, ProjectCommand{
						ProjectName: project.Name,
						ProjectDir:  project.Dir,
						Commands:    []string{command},
					})
				}
			}
		}
		return commandsPerProject, nil
	default:
		return []ProjectCommand{}, fmt.Errorf("unsupported event type: %T", event)
	}
}

func ConvertBitBucketEventToCommands(event models.Event, impactedProjects []Project) ([]ProjectCommand, error) {
	commandsPerProject := make([]ProjectCommand, 0)

	switch event.(type) {
	case models.PullRequestEvent:
		event := event.(models.PullRequestEvent)
		for _, project := range impactedProjects {
			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName: project.Name,
					ProjectDir:  project.Dir,
					Commands:    project.WorkflowConfiguration.OnCommitToDefault,
				})
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName: project.Name,
					ProjectDir:  project.Dir,
					Commands:    project.WorkflowConfiguration.OnPullRequestPushed,
				})
			} else if event.Action == "closed" {
				commandsPerProject = append(commandsPerProject, ProjectCommand{
					ProjectName: project.Name,
					ProjectDir:  project.Dir,
					Commands:    project.WorkflowConfiguration.OnPullRequestClosed,
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
					commandsPerProject = append(commandsPerProject, ProjectCommand{
						ProjectName: project.Name,
						ProjectDir:  project.Dir,
						Commands:    []string{command},
					})
				}
			}
		}
		return commandsPerProject, nil
	default:
		return []ProjectCommand{}, fmt.Errorf("unsupported event type: %T", event)
	}
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
	repoOwner    string
	projectName  string
	projectDir   string
	repoName     string
	prManager    github.PullRequestManager
	lock         utils.ProjectLock
	configDigger *DiggerConfig
}

func (d DiggerExecutor) LockId() string {
	return d.repoName + "#" + d.projectName
}

func (d DiggerExecutor) Plan(prNumber int) {

	terraformExecutor := terraform.Terraform{WorkingDir: path.Join(d.workingDir, d.projectDir)}

	res, err := d.lock.Lock(d.LockId(), prNumber)
	if err != nil {
		log.Fatalf("Error locking project: %v", err)
	}
	if res {
		isNonEmptyPlan, stdout, stderr, err := terraformExecutor.Plan()

		if err != nil {
			log.Fatalf("Error executing plan: %v", err)
		}
		plan := cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
		comment := "Plan for **" + d.LockId() + "**\n" + plan
		d.prManager.PublishComment(prNumber, comment)
	}

}

func (d DiggerExecutor) Apply(prNumber int) {
	terraformExecutor := terraform.Terraform{WorkingDir: path.Join(d.workingDir, d.projectDir)}
	if res, _ := d.lock.Lock(d.LockId(), prNumber); res {
		stdout, stderr, err := terraformExecutor.Apply()
		applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
		comment := "Apply for **" + d.LockId() + "**\n" + applyOutput
		d.prManager.PublishComment(prNumber, comment)
		d.lock.Unlock(d.LockId(), prNumber)
	}

}

func (d DiggerExecutor) Unlock(prNumber int) {
	d.lock.ForceUnlock(d.LockId(), prNumber)

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
