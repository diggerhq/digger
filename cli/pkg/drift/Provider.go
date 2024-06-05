package drift

import (
	"fmt"
	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	"github.com/diggerhq/digger/libs/orchestrator"
	"os"
)

type DriftNotificationProvider interface {
	Get(prService orchestrator.PullRequestService) (core_drift.Notification, error)
}

type DriftNotificationProviderBasic struct{}

func (d DriftNotificationProviderBasic) Get(prService orchestrator.PullRequestService) (core_drift.Notification, error) {
	slackNotificationUrl := os.Getenv("INPUT_DRIFT_DETECTION_SLACK_NOTIFICATION_URL")
	var notification core_drift.Notification
	if slackNotificationUrl != "" {
		notification = SlackNotification{slackNotificationUrl}
	} else {
		return nil, fmt.Errorf("could not identify drift mode, please specify slack or github")
	}
	return notification, nil
}
