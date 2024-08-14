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
	title := fmt.Sprintf("Drift detected in project: %v", projectName)
	message := fmt.Sprintf(":bangbang: Drift detected in digger project %v details below: \n\n```\n%v\n```", projectName, plan)
	existingIssues, err := (*ghi.GithubService).ListIssues()
	if err != nil {
		log.Printf("failed to retrive issues: %v", err)
		return fmt.Errorf("failed to retrive issues: %v", err)
	}

	theIssue, exists := lo.Find(existingIssues, func(item *orchestrator.Issue) bool {
		return item.Title == title
	})
	if exists {
		log.Printf("Issue found: %v", theIssue)
		_, err := (*ghi.GithubService).UpdateIssue(theIssue.ID, theIssue.Title, theIssue.Body)
		if err != nil {
			log.Printf("error while updating issue: %v", err)
		}
		return err
	} else {
		log.Printf("Issue NOT found: %v", theIssue)
		_, err := (*ghi.GithubService).PublishIssue(title, message)
		if err != nil {
			log.Printf("error while publishing issue: %v", err)
		}
		return err
	}
	return nil
}
