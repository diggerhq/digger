package drift

import (
	"fmt"
	orchestrator "github.com/diggerhq/digger/libs/ci"
	"github.com/samber/lo"
	"log"
)

type GithubIssueNotification struct {
	GithubService   *orchestrator.PullRequestService
	RelatedPrNumber *int64
}

func (ghi GithubIssueNotification) Send(projectName string, plan string) error {
	log.Printf("Info: Sending drift notification regarding project: %v", projectName)
	title := fmt.Sprintf("Drift detected in project: %v", projectName)
	message := fmt.Sprintf(":bangbang: Drift detected in digger project %v details below: \n\n```\n%v\n```", projectName, plan)
	existingIssues, err := (*ghi.GithubService).ListIssues()
	if err != nil {
		log.Printf("failed to retrieve issues: %v", err)
		return fmt.Errorf("failed to retrieve issues: %v", err)
	}

	theIssue, exists := lo.Find(existingIssues, func(item *orchestrator.Issue) bool {
		return item.Title == title
	})
	if exists {
		_, err := (*ghi.GithubService).UpdateIssue(theIssue.ID, theIssue.Title, message)
		if err != nil {
			log.Printf("error while updating issue: %v", err)
		}
		return err
	} else {
		labels := []string{"digger"}
		_, err := (*ghi.GithubService).PublishIssue(title, message, &labels)
		if err != nil {
			log.Printf("error while publishing issue: %v", err)
		}
		return err
	}
	return nil
}
