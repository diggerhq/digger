package drift

import (
	"fmt"
	orchestrator "github.com/diggerhq/digger/libs/orchestrator"
)

type GithubIssueNotification struct {
	GithubService   *orchestrator.PullRequestService
	RelatedPrNumber *int64
}

func (ghi GithubIssueNotification) Send(projectName string, plan string) error {
	title := fmt.Sprintf("Drift detected in project: %v", projectName)
	message := fmt.Sprintf(":bangbang: Drift detected in digger project %v details below: \n\n```\n%v\n```", projectName, plan)
	_, err := (*ghi.GithubService).PublishIssue(title, message)
	return err
}
