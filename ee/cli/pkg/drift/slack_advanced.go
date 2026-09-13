package drift

import (
	"fmt"
	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	"github.com/diggerhq/digger/cli/pkg/drift"
	"github.com/diggerhq/digger/libs/comment_utils"
	"strings"
)

type SlackAdvancedAggregatedNotificationWithAiSummary struct {
	Url          string
	RepoFullName string
	projectNames []string
	lastChanges  map[string]*core_drift.LastChange
}

func NewSlackAdvancedAggregatedNotificationWithAiSummary(url string) *SlackAdvancedAggregatedNotificationWithAiSummary {
	return &SlackAdvancedAggregatedNotificationWithAiSummary{
		Url:          url,
		projectNames: make([]string, 0),
		lastChanges:  make(map[string]*core_drift.LastChange),
	}
}

func (slack *SlackAdvancedAggregatedNotificationWithAiSummary) SendNotificationForProject(projectName string, repoFullName string, plan string, lastChange *core_drift.LastChange) error {
	slack.projectNames = append(slack.projectNames, projectName)
	slack.lastChanges[projectName] = lastChange
	slack.RepoFullName = repoFullName
	return nil
}

func (slack *SlackAdvancedAggregatedNotificationWithAiSummary) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	message := fmt.Sprintf(
		":warning: *Error While Drift Processing* :warning:\n\n"+
			"*Project:* `%s`\n"+
			"*Repository:* `%s`\n\n"+
			"*Error Details:*\n```\n%v\n```\n\n"+
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
		if lc := slack.lastChanges[projectName]; lc != nil {
			projectList.WriteString(fmt.Sprintf("• `%s` — last change by %s (`%s`, %s)\n", projectName, lc.Author, lc.Commit, lc.When))
		} else {
			projectList.WriteString(fmt.Sprintf("• `%s`\n", projectName))
		}
	}

	if len(slack.projectNames) > 50 {
		projectList.WriteString("_and more..._\n")
	}

	message := fmt.Sprintf(
		":warning: *Drift Detected* :warning:\n\n"+
			"*Repository:* `%s`\n\n"+
			"*Affected Digger Projects:*\n%s\n"+
			"<<%s|View Workflow>>",
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
