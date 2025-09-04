package drift

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/libs/comment_utils"
	"strings"
)

type SlackAdvancedAggregatedNotificationWithAiSummary struct {
	Url          string
	RepoFullName string
	projectNames []string
}

func NewSlackAdvancedAggregatedNotificationWithAiSummary(url string) *SlackAdvancedAggregatedNotificationWithAiSummary {
	return &SlackAdvancedAggregatedNotificationWithAiSummary{
		Url:          url,
		projectNames: make([]string, 0),
	}
}

func (slack *SlackAdvancedAggregatedNotificationWithAiSummary) SendNotificationForProject(projectName string, repoFullName string, plan string) error {
	slack.projectNames = append(slack.projectNames, projectName)
	slack.RepoFullName = repoFullName
	return nil
}

func (slack *SlackAdvancedAggregatedNotificationWithAiSummary) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	message := fmt.Sprintf(
		":rotating_light: *Error While Drift Processing* :rotating_light:\n\n"+
			":file_folder: *Project:* `%s`\n"+
			":books: *Repository:* `%s`\n\n"+
			":warning: *Error Details:*\n```\n%v\n```\n\n"+
			"_Please check the workflow logs for more information._",
		projectName, repoFullName, err,
	)

	return drift.SendSlackMessage(slack.Url, message)
}

func (slack *SlackAdvancedAggregatedNotificationWithAiSummary) Flush() error {
	if len(slack.projectNames) == 0 {
		return nil
	}
	var projectNamesCompact = slack.projectNames
	if len(slack.projectNames) > 50 {
		projectNamesCompact = slack.projectNames[:50]
	}
	var projectList strings.Builder
	for _, projectName := range projectNamesCompact {
		projectList.WriteString(fmt.Sprintf("• `%s`\n", projectName))
	}

	if len(slack.projectNames) > 50 {
		projectList.WriteString("_and more..._\n")
	}

	message := fmt.Sprintf(
		":warning: *Drift Detected* :warning:\n\n"+
			"*Repository:* `%s`\n\n"+
			"*Affected Digger Projects:*\n%s\n"+
			":link: <<%s|View Workflow>>",
		slack.RepoFullName,
		projectList.String(),
		comment_utils.GetWorkflowUrl(),
	)

	err := drift.SendSlackMessage(slack.Url, message)
	if err != nil {
		return err
	}
	return nil
}
