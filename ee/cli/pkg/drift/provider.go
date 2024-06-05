package drift

import (
	"fmt"
	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/libs/orchestrator"
	"os"
)

type DriftNotificationProviderAdvanced struct{}

func (d DriftNotificationProviderAdvanced) Get(prService orchestrator.PullRequestService) (core_drift.Notification, error) {
	slackNotificationUrl := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL")
	DriftAsGithubIssues := os.Getenv("INPUT_DRIFT_GITHUB_ISSUES")
	var notification core_drift.Notification
	if slackNotificationUrl != "" {
		notification = drift.SlackNotification{slackNotificationUrl}
	} else if DriftAsGithubIssues != "" {
		notification = GithubIssueNotification{GithubService: &prService}
	} else {
		return nil, fmt.Errorf("could not identify drift mode, please specify slack or github")
	}
	return notification, nil
}
