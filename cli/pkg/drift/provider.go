package drift

import (
	"fmt"
	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	"github.com/diggerhq/digger/libs/ci"
	"os"
)

type DriftNotificationProvider interface {
	Get(prService ci.PullRequestService) (core_drift.Notification, error)
}

type DriftNotificationProviderBasic struct{}

func (d DriftNotificationProviderBasic) Get(prService ci.PullRequestService) (core_drift.Notification, error) {
	slackNotificationUrl := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL")
	slackBotToken := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_BOT_TOKEN")
	slackChannel := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_CHANNEL")
	githubIssues := os.Getenv("INPUT_DRIFT_GITHUB_ISSUES")
	if (slackBotToken != "") != (slackChannel != "") {
		return nil, fmt.Errorf("threaded slack notifications need both INPUT_DRIFT_DETECTION_SLACK_BOT_TOKEN and INPUT_DRIFT_DETECTION_SLACK_CHANNEL, but only one is set")
	}
	var notification core_drift.Notification
	if slackNotificationUrl != "" || (slackBotToken != "" && slackChannel != "") {
		notification = &SlackNotification{Url: slackNotificationUrl, BotToken: slackBotToken, Channel: slackChannel}
	} else if githubIssues != "" {
		notification = &GithubIssueNotification{GithubService: &prService}
	} else {
		return nil, fmt.Errorf("could not identify drift mode, please specify using INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL (or INPUT_DRIFT_DETECTION_SLACK_BOT_TOKEN + INPUT_DRIFT_DETECTION_SLACK_CHANNEL for threaded plans) or INPUT_DRIFT_GITHUB_ISSUES")
	}
	return notification, nil
}
