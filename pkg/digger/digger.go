package digger

import (
	"digger/pkg/aws"
	"digger/pkg/github"
	"digger/pkg/models"
	"digger/pkg/terraform"
	"digger/pkg/utils"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
)

func ProcessGitHubContext(parsedGhContext *models.Github, ghEvent map[string]interface{}, diggerConfig *DiggerConfig, prManager github.PullRequestManager, eventName string, dynamoDbLock *aws.DynamoDbLock, workingDir string) error {
	var parsedGhEvent interface{}
	var impactedProjects []Project
	var prNumber int
	if eventName == "pull_request" {
		parsedGhEvent := models.PullRequestEvent{}

		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing PullRequestEvent: %v", err)
		}
		prNumber = parsedGhEvent.Number
		changedFiles, err := prManager.GetChangedFiles(prNumber)

		if err != nil {
			return fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	} else if eventName == "issue_comment" {
		parsedGhEvent := models.IssueCommentEvent{}
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			log.Fatalf("error parsing IssueCommentEvent: %v", err)
		}
		prNumber = parsedGhEvent.Issue.Number
		requestedProject := parseProjectName(parsedGhEvent.Comment.Body)
		if requestedProject != "" {
			impactedProjects = diggerConfig.GetProjects(requestedProject)
		} else {
			changedFiles, err := prManager.GetChangedFiles(prNumber)
			if err != nil {
				log.Fatalf("Could not get changed files")
			}
			impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		}
	}

	commandsToRunPerProject, err := ConvertGithubEventToCommands(parsedGhEvent, diggerConfig, impactedProjects)

	if err != nil {
		return fmt.Errorf("error converting github event to commands: %v", err)
	}

	lockAcquisitionSuccess := true
	for project, commands := range commandsToRunPerProject {
		for _, command := range commands {
			switch command {
			case "digger plan":
				utils.SendUsageRecord(parsedGhContext.RepositoryOwner, eventName, "plan")
				projectLock := &utils.ProjectLockImpl{
					InternalLock: dynamoDbLock,
					PrManager:    prManager,
					ProjectName:  project,
					RepoName:     parsedGhContext.Repository,
				}
				diggerExecutor := DiggerExecutor{
					workingDir,
					parsedGhContext.RepositoryOwner,
					parsedGhContext.Repository,
					project,
					parsedGhContext.Repository,
					prManager,
					projectLock,
					diggerConfig,
				}
				diggerExecutor.Apply(prNumber)
			case "digger apply":
				utils.SendUsageRecord(parsedGhContext.RepositoryOwner, eventName, "apply")
				projectLock := &utils.ProjectLockImpl{
					InternalLock: dynamoDbLock,
					PrManager:    prManager,
					ProjectName:  project,
					RepoName:     parsedGhContext.Repository,
				}
				diggerExecutor := DiggerExecutor{
					workingDir,
					parsedGhContext.RepositoryOwner,
					parsedGhContext.Repository,
					project,
					parsedGhContext.Repository,
					prManager,
					projectLock,
					diggerConfig,
				}
				diggerExecutor.Plan(prNumber)
			case "digger unlock":
				utils.SendUsageRecord(parsedGhContext.RepositoryOwner, eventName, "unlock")
				lockID := fmt.Sprintf("%s#%s", parsedGhContext.Repository, project)
				projectLock := utils.ProjectLockImpl{InternalLock: dynamoDbLock, PrManager: prManager, ProjectName: project, RepoName: parsedGhContext.Repository}
				_, err := projectLock.Unlock(lockID, prNumber)
				if err != nil {
					return err
				}
			case "digger lock":
				utils.SendUsageRecord(parsedGhContext.RepositoryOwner, eventName, "lock")
				lockID := fmt.Sprintf("%s#%s", parsedGhContext.Repository, project)
				projectLock := utils.ProjectLockImpl{InternalLock: dynamoDbLock, PrManager: prManager, ProjectName: project, RepoName: parsedGhContext.Repository}
				isLocked, err := projectLock.Lock(lockID, prNumber)
				if err != nil {
					log.Fatalf("Failed to aquire lock: " + lockID)
				}
				if !isLocked {
					lockAcquisitionSuccess = false
				}
			}
		}
	}

	if !lockAcquisitionSuccess {
		os.Exit(1)
	}
	return nil
}

func GetGitHubContext(ghContext string) (models.Github, error) {
	var parsedGhContext models.Github
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return models.Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

func ConvertGithubEventToCommands(event interface{}, diggerConfig *DiggerConfig, impactedProjects []Project) (map[string][]string, error) {
	commandsPerProject := make(map[string][]string)

	switch event.(type) {
	default:
		return map[string][]string{}, fmt.Errorf("unsupported event type: %T", event)
	case models.PullRequestEvent:
		event := event.(models.PullRequestEvent)
		for _, project := range impactedProjects {
			workflowConfiguration := diggerConfig.GetWorkflowConfiguration(project.Name)
			if event.Action == "closed" && event.PullRequest.Merged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
				commandsPerProject[project.Name] = workflowConfiguration.OnPullRequestPushed
			} else if event.Action == "opened" || event.Action == "reopened" || event.Action == "synchronize" {
				commandsPerProject[project.Name] = workflowConfiguration.OnPullRequestPushed
			} else if event.Action == "closed" {
				commandsPerProject[project.Name] = workflowConfiguration.OnPullRequestClosed
			}
		}
		return commandsPerProject, nil
	case models.IssueCommentEvent:
		event := event.(models.IssueCommentEvent)
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock"}

		for _, command := range supportedCommands {
			if strings.Contains(event.Comment.Body, command) {
				for _, project := range impactedProjects {
					commandsPerProject[project.Name] = []string{command}
				}
			}
		}
		return commandsPerProject, nil
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

func (d DiggerExecutor) Plan(prNumber int) {
	lockId := d.repoName + "#" + d.projectName

	terraformExecutor := terraform.Terraform{WorkingDir: path.Join(d.workingDir, d.projectDir)}

	res, err := d.lock.Lock(lockId, prNumber)
	if err != nil {
		log.Fatalf("Error locking project: %v", err)
	}
	if res {
		isNonEmptyPlan, stdout, stderr, err := terraformExecutor.Plan()

		if err != nil {
			log.Fatalf("Error executing plan: %v", err)
		}
		plan := cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
		comment := "Plan for **" + lockId + "**\n" + plan
		d.prManager.PublishComment(prNumber, comment)
	}

}

func (d DiggerExecutor) Apply(prNumber int) {
	projectName := d.projectName
	lockId := d.repoName + "#" + projectName
	terraformExecutor := terraform.Terraform{WorkingDir: path.Join(d.workingDir, d.projectDir)}
	if res, _ := d.lock.Lock(lockId, prNumber); res {
		stdout, stderr, err := terraformExecutor.Apply()
		applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
		comment := "Apply for **" + lockId + "**\n" + applyOutput
		d.prManager.PublishComment(prNumber, comment)
		d.lock.Unlock(lockId, prNumber)
	}

}

func (d DiggerExecutor) Unlock(prNumber int) {
	lockId := d.repoName + "#" + d.projectName
	d.lock.ForceUnlock(lockId, prNumber)

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
