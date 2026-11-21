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
	slackNotificationAdvancedUrl := os.Getenv("INPUT_DRIFT_DETECTION_ADVANCED_SLACK_NOTIFICATION_URL")
	DriftAsGithubIssues := os.Getenv("INPUT_DRIFT_GITHUB_ISSUES")
	githubActionOutput := os.Getenv("INPUT_DRIFT_DETECTION_GITHUB_ACTION_OUTPUT")
	var notification core_drift.Notification
	if slackNotificationUrl != "" {
		notification = &ce_drift.SlackNotification{Url: slackNotificationUrl}
	} else if slackNotificationAdvancedUrl != "" {
		notification = NewSlackAdvancedAggregatedNotificationWithAiSummary(slackNotificationAdvancedUrl)
	} else if DriftAsGithubIssues != "" {
		notification = &ce_drift.GithubIssueNotification{GithubService: &prService}
	} else if githubActionOutput == "true" {
		notification = &ce_drift.GithubActionOutputNotification{}
	} else {
		return nil, fmt.Errorf("could not identify drift mode, please specify using env variable INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL, INPUT_DRIFT_DETECTION_ADVANCED_SLACK_NOTIFICATION_URL, INPUT_DRIFT_GITHUB_ISSUES, or INPUT_DRIFT_DETECTION_GITHUB_ACTION_OUTPUT")
	}
	return notification, nil
}
