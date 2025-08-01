package drift

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/libs/comment_utils"
)

type SlackAdvancedAggregatedNotificationWithAiSummary struct {
	Url          string
	RepoFullName string
	projectNames []string
}

func NewSlackAdvancedAggregatedNotificationWithAiSummary(url string) SlackAdvancedAggregatedNotificationWithAiSummary {
	return SlackAdvancedAggregatedNotificationWithAiSummary{
		Url:          url,
		projectNames: make([]string, 0),
	}
}

func (slack SlackAdvancedAggregatedNotificationWithAiSummary) SendNotificationForProject(projectName string, repoFullName string, plan string) error {
	slack.projectNames = append(slack.projectNames, projectName)
	slack.RepoFullName = repoFullName
	return nil
}

func (slack SlackAdvancedAggregatedNotificationWithAiSummary) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	message := fmt.Sprintf(":red_circle: Encountered an error while processing drift, project: %v, repo: %v details below: \n\n```\n%v\n```", projectName, repoFullName, err)
	return drift.SendSlackMessage(slack.Url, message)
}

func (slack SlackAdvancedAggregatedNotificationWithAiSummary) Flush() error {
	message := fmt.Sprintf(":bangbang: Drift detected in repo %v. digger projects: \n\n", slack.RepoFullName)
	for _, projectName := range slack.projectNames {
		message = message + fmt.Sprintf("- %v \n", projectName)
	}
	message = message + "\n\n"
	message = message + fmt.Sprintf("workflow url: %v", comment_utils.GetWorkflowUrl())
	err := drift.SendSlackMessage(slack.Url, message)
	if err != nil {
		return err
	}
	return nil
}
