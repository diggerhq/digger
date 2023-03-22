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

func ProcessGitHubContext(parsedGhContext *models.Github, ghEvent map[string]interface{}, diggerConfig *DiggerConfig, prManager github.PullRequestManager, eventName string, dynamoDbLock aws.Lock, workingDir string) error {
	if parsedGhContext.EventName == "pull_request" {
		var parsedGhEvent models.PullRequestEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing PullRequestEvent: %v", err)
		}

		if parsedGhEvent.PullRequest.Merged {
			println("PR was merged")
		}
		prStatesToLock := []string{"reopened", "opened", "synchronize"}
		prStatesToUnlock := []string{"closed"}

		if contains(prStatesToLock, parsedGhEvent.Action) {
			err := processNewPullRequest(diggerConfig, prManager, parsedGhContext.RepositoryOwner, parsedGhContext.Repository, eventName, parsedGhEvent.Number, dynamoDbLock)
			if err != nil {
				return err
			}
		} else if contains(prStatesToUnlock, parsedGhEvent.Action) {
			err := processClosedPullRequest(diggerConfig, prManager, parsedGhContext.RepositoryOwner, parsedGhContext.Repository, eventName, parsedGhEvent.Number, dynamoDbLock)
			if err != nil {
				return err
			}
		}

	} else if parsedGhContext.EventName == "issue_comment" {
		var parsedGhEvent models.IssueCommentEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			log.Fatalf("error parsing IssueCommentEvent: %v", err)
		}

		//fmt.Printf("comment: %s\n", parsedGhEvent.Comment.Body)
		//fmt.Printf("issue number: %d\n", parsedGhEvent.Issue.Number)

		err = processPullRequestComment(diggerConfig, prManager, eventName, parsedGhContext.RepositoryOwner, parsedGhContext.Repository, parsedGhEvent.Issue.Number, parsedGhEvent.Comment.Body, dynamoDbLock, workingDir)

		if err != nil {
			log.Fatalf("error processing pull request comment: %v", err)
		}

		log.Printf("Issue PR #%v was commented on", parsedGhEvent.Issue.Number)
	}
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func GetGitHubContext(ghContext string) (models.Github, error) {
	var parsedGhContext models.Github
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return models.Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

func processNewPullRequest(diggerConfig *DiggerConfig, prManager github.PullRequestManager, repoOwner string, repoName string, eventName string, prNumber int, dynamoDbLock aws.Lock) error {
	utils.SendUsageRecord(repoOwner, eventName, "lock")
	lockAcquisitionSuccess := true

	changedFiles, err := prManager.GetChangedFiles(prNumber)
	if err != nil {
		log.Fatalf("Could not get changed files")
	}

	modifiedProjects := diggerConfig.GetModifiedProjects(changedFiles)
	for _, project := range modifiedProjects {
		projectName := project.Name
		lockID := fmt.Sprintf("%s#%s", repoName, projectName)
		projectLock := utils.ProjectLockImpl{InternalLock: dynamoDbLock, PrManager: prManager, ProjectName: projectName, RepoName: repoName}
		isLocked, err := projectLock.Lock(lockID, prNumber)
		if err != nil {
			log.Fatalf("Failed to aquire lock: " + lockID)
		}

		if !isLocked {
			lockAcquisitionSuccess = false
		}
	}
	if !lockAcquisitionSuccess {
		os.Exit(1)
	}
	println("Processing new PR")
	return nil
}

func processClosedPullRequest(diggerConfig *DiggerConfig, prManager github.PullRequestManager, repoOwner string, repoName string, eventName string, prNumber int, dynamoDbLock aws.Lock) error {
	utils.SendUsageRecord(repoOwner, eventName, "lock")

	files, err := prManager.GetChangedFiles(prNumber)
	if err != nil {
		return err
	}
	for _, project := range diggerConfig.GetModifiedProjects(files) {
		lockID := fmt.Sprintf("%s#%s", repoName, project.Name)
		projectLock := utils.ProjectLockImpl{InternalLock: dynamoDbLock, PrManager: prManager, ProjectName: project.Name, RepoName: repoName}
		_, err := projectLock.Unlock(lockID, prNumber)
		if err != nil {
			return err
		}
	}

	return nil
}

func processPullRequestComment(diggerConfig *DiggerConfig, prManager github.PullRequestManager, eventName string, repoOwner string, repoName string, prNumber int, commentBody string, dynamoDbLock aws.Lock, workingDir string) error {
	print("Processing PR comment")
	requestedProject := parseProjectName(commentBody)
	var impactedProjects []Project
	if requestedProject != "" {
		impactedProjects = diggerConfig.GetProjects(requestedProject)
	} else {
		changedFiles, err := prManager.GetChangedFiles(prNumber)
		if err != nil {
			log.Fatalf("Could not get changed files")
		}
		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	}

	trimmedComment := strings.TrimSpace(commentBody)
	if trimmedComment == "digger plan" {
		for _, p := range impactedProjects {
			projectLock := &utils.ProjectLockImpl{
				InternalLock: dynamoDbLock,
				PrManager:    prManager,
				ProjectName:  p.Name,
				RepoName:     repoName,
			}
			diggerExecutor := DiggerExecutor{
				workingDir,
				repoOwner,
				repoName,
				impactedProjects,
				prManager,
				projectLock,
				diggerConfig,
			}
			diggerExecutor.Plan(eventName, prNumber)
		}

	} else if trimmedComment == "digger apply" {
		for _, p := range impactedProjects {
			projectLock := &utils.ProjectLockImpl{
				InternalLock: dynamoDbLock,
				PrManager:    prManager,
				ProjectName:  p.Name,
				RepoName:     repoName,
			}
			diggerExecutor := DiggerExecutor{
				workingDir,
				repoOwner,
				repoName,
				impactedProjects,
				prManager,
				projectLock,
				diggerConfig,
			}
			diggerExecutor.Apply(eventName, prNumber)

		}

	} else if trimmedComment == "digger unlock" {
		for _, p := range impactedProjects {
			projectLock := &utils.ProjectLockImpl{
				InternalLock: dynamoDbLock,
				PrManager:    prManager,
				ProjectName:  p.Name,
				RepoName:     repoName,
			}
			diggerExecutor := DiggerExecutor{
				workingDir,
				repoOwner,
				repoName,
				impactedProjects,
				prManager,
				projectLock,
				diggerConfig,
			}
			diggerExecutor.Unlock(eventName, prNumber)
		}
	}
	return nil
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
	workingDir       string
	repoOwner        string
	repoName         string
	impactedProjects []Project
	prManager        github.PullRequestManager
	lock             utils.ProjectLock
	configDigger     *DiggerConfig
}

func (d DiggerExecutor) Plan(triggerEvent string, prNumber int) {
	utils.SendUsageRecord(d.repoOwner, triggerEvent, "plan")

	for _, project := range d.impactedProjects {
		projectName := project.Name
		lockId := d.repoName + "#" + projectName

		directory := project.Dir
		terraformExecutor := terraform.Terraform{WorkingDir: path.Join(d.workingDir, directory)}

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
}

func (d DiggerExecutor) Apply(triggerEvent string, prNumber int) {
	utils.SendUsageRecord(d.repoOwner, triggerEvent, "apply")
	for _, project := range d.impactedProjects {
		projectName := project.Name
		lockId := d.repoName + "#" + projectName
		directory := project.Dir
		terraformExecutor := terraform.Terraform{WorkingDir: directory}

		if res, _ := d.lock.Lock(lockId, prNumber); res {
			stdout, stderr, err := terraformExecutor.Apply()
			applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
			comment := "Apply for **" + lockId + "**\n" + applyOutput
			d.prManager.PublishComment(prNumber, comment)
			d.lock.Unlock(lockId, prNumber)
		}
	}
}

func (d DiggerExecutor) Unlock(triggerEvent string, prNumber int) {
	utils.SendUsageRecord(d.repoOwner, triggerEvent, "unlock")
	for _, project := range d.impactedProjects {
		projectName := project.Name
		lockId := d.repoName + "#" + projectName
		d.lock.ForceUnlock(lockId, prNumber)
	}
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
