package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/appengine/log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

func main() {
	diggerConfig, err := NewDiggerConfig()
	if err != nil {
		print("Failed to read digger config.")
		os.Exit(1)
	}
	sess := session.Must(session.NewSession())
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := DynamoDbLock{DynamoDb: dynamoDb}

	ghToken := os.Getenv("GITHUB_TOKEN")

	ghContext := os.Getenv("GITHUB_CONTEXT")

	parsedGhContext, err := getGitHubContext(ghContext)
	if ghContext == "" {
		print("GITHUB_CONTEXT is not defined")
		os.Exit(1)
	}

	tf := Terraform{}

	ghEvent := parsedGhContext.Event
	eventName := parsedGhContext.EventName
	repoOwner := parsedGhContext.RepositoryOwner
	repositoryName := parsedGhContext.Repository
	githubPrService := NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	err = processGitHubContext(&parsedGhContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, &tf)
	if err != nil {
		print(err)
		os.Exit(1)
	}
}

func processGitHubContext(parsedGhContext *Github, ghEvent map[string]interface{}, diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, tf TerraformExecutor) error {

	if parsedGhContext.EventName == "pull_request" {

		var parsedGhEvent PullRequestEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing PullRequestEvent: %v", err)
		}

		if parsedGhEvent.PullRequest.Merged {
			print("PR was merged")
		}
		prStatesToLock := []string{"reopened", "opened", "synchronize"}
		prStatesToUnlock := []string{"closed"}

		if contains(prStatesToLock, parsedGhEvent.Action) {
			err := processNewPullRequest(diggerConfig, prManager, eventName, dynamoDbLock, parsedGhEvent.Number)
			if err != nil {
				return err
			}
		} else if contains(prStatesToUnlock, parsedGhEvent.Action) {
			err := processClosedPullRequest(diggerConfig, prManager, eventName, dynamoDbLock, parsedGhEvent.Number)
			if err != nil {
				return err
			}
		}

	} else if parsedGhContext.EventName == "issue_comment" {
		var parsedGhEvent IssueCommentEvent
		err := mapstructure.Decode(ghEvent, &parsedGhEvent)
		if err != nil {
			return fmt.Errorf("error parsing IssueCommentEvent: %v", err)
		}
		print("Issue PR #" + string(rune(parsedGhEvent.Comment.Issue.Number)) + " was commented on")

		err = processPullRequestComment(diggerConfig, prManager, eventName, dynamoDbLock, tf, parsedGhEvent.Comment.Issue.Number, parsedGhEvent.Comment.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func getGitHubContext(ghContext string) (Github, error) {
	var parsedGhContext Github
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func processNewPullRequest(diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, prNumber int) error {
	print("Processing new PR")
	return nil
}

func processClosedPullRequest(diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, prNumber int) error {
	print("Processing closed PR")
	return nil
}

func processPullRequestComment(diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, tf TerraformExecutor, prNumber int, commentBody string) error {
	print("Processing PR comment")
	trimmedComment := strings.TrimSpace(commentBody)
	if trimmedComment == "digger plan" {
		_, _, _, err := tf.Plan()
		if err != nil {
			return err
		}

	} else if trimmedComment == "digger apply" {
		_, _, err := tf.Apply()
		if err != nil {
			return err
		}

	} else if trimmedComment == "digger unlock" {

	}
	return nil
}

type UsageRecord struct {
	UserId    interface{} `json:"userid"`
	EventName string      `json:"event_name"`
	Action    string      `json:"action"`
	Token     string      `json:"token"`
}

func sendUsageRecord(repoOwner string, eventName string, action string) {
	h := sha256.New()
	h.Write([]byte(repoOwner))
	sha := h.Sum(nil)
	shaStr := hex.EncodeToString(sha)
	payload := UsageRecord{
		UserId:    shaStr,
		EventName: eventName,
		Action:    action,
		Token:     os.Getenv("USAGE_TOKEN"),
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Errorf(context.Background(), "Error marshalling usage record: %v", err)
		return
	}
	req, _ := http.NewRequest("POST", os.Getenv("USAGE_URL"), bytes.NewBuffer(jsonData))

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf(context.Background(), "Error sending usage record: %v", err)
		return
	}
	defer resp.Body.Close()
}

type DiggerExecutor struct {
	repoOwner        string
	repoName         string
	impactedProjects []Project
	prManager        PullRequestManager
	lock             ProjectLock
	configDigger     DiggerConfig
}

func (d DiggerExecutor) Plan(triggerEvent string, prNumber int) {
	sendUsageRecord(d.repoOwner, triggerEvent, "plan")

	for _, project := range d.impactedProjects {
		projectName := project.Name
		lockId := d.repoName + "#" + projectName
		directory := project.Dir
		terraformExecutor := Terraform{directory}
		if res, _ := d.lock.Lock(lockId, prNumber); res {
			isNonEmptyPlan, stdout, stderr, err := terraformExecutor.Plan()
			plan := cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
			comment := "Plan for **" + lockId + "**\n" + plan
			d.prManager.PublishComment(prNumber, comment)
		}
	}
}

func (d DiggerExecutor) Apply(triggerEvent string, prNumber int) {
	sendUsageRecord(d.repoOwner, triggerEvent, "apply")
	for _, project := range d.impactedProjects {
		projectName := project.Name
		lockId := d.repoName + "#" + projectName
		directory := project.Dir
		terraformExecutor := Terraform{directory}
		if res, _ := d.lock.Lock(lockId, 0); res {
			stdout, stderr, err := terraformExecutor.Apply()
			applyOutput := cleanupTerraformApply(true, err, stdout, stderr)
			comment := "Apply for **" + lockId + "**\n" + applyOutput
			d.prManager.PublishComment(prNumber, comment)
		}
	}
}

func (d DiggerExecutor) Unlock(triggerEvent string, prNumber int) {
	sendUsageRecord(d.repoOwner, triggerEvent, "unlock")
	for _, project := range d.impactedProjects {
		projectName := project.Name
		lockId := d.repoName + "#" + projectName
		d.lock.ForceUnlock(lockId, prNumber)
	}
}

func cleanupTerraformOutput(nonEmptyOutput bool, planError error, stdout string, stderr string, regexStr string) string {
	var errorStr, result, start string
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
