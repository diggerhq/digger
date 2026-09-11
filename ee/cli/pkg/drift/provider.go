package drift

import (
	"fmt"
	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	ce_drift "github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/libs/ci"
	"os"
)

type DriftNotificationProviderAdvanced struct{}

func (d DriftNotificationProviderAdvanced) Get(prService ci.PullRequestService) (core_drift.Notification, error) {
	slackNotificationUrl := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL")
	slackBotToken := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_BOT_TOKEN")
	slackChannel := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_CHANNEL")
	slackNotificationAdvancedUrl := os.Getenv("INPUT_DRIFT_DETECTION_ADVANCED_SLACK_NOTIFICATION_URL")
	DriftAsGithubIssues := os.Getenv("INPUT_DRIFT_GITHUB_ISSUES")
	if (slackBotToken != "") != (slackChannel != "") {
		return nil, fmt.Errorf("threaded slack notifications need both INPUT_DRIFT_DETECTION_SLACK_BOT_TOKEN and INPUT_DRIFT_DETECTION_SLACK_CHANNEL, but only one is set")
	}
	var notification core_drift.Notification
	if slackNotificationUrl != "" {
		notification = &ce_drift.SlackNotification{Url: slackNotificationUrl, BotToken: slackBotToken, Channel: slackChannel}
	} else if slackNotificationAdvancedUrl != "" {
		// advanced (aggregated + AI summary) keeps precedence over the
		// bot-token mode so existing EE setups don't silently change behaviour
		notification = NewSlackAdvancedAggregatedNotificationWithAiSummary(slackNotificationAdvancedUrl)
	} else if slackBotToken != "" && slackChannel != "" {
		notification = &ce_drift.SlackNotification{BotToken: slackBotToken, Channel: slackChannel}
	} else if DriftAsGithubIssues != "" {
		notification = &ce_drift.GithubIssueNotification{GithubService: &prService}
	} else {
		return nil, fmt.Errorf("could not identify drift mode, please specify using env variable INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL, INPUT_DRIFT_DETECTION_ADVANCED_SLACK_NOTIFICATION_URL or INPUT_DRIFT_GITHUB_ISSUES")
	}
	return notification, nil
}
