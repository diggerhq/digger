package drift

import (
	"fmt"
	"log"

	orchestrator "github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/samber/lo"
)

type GithubIssueNotification struct {
	GithubService   *orchestrator.PullRequestService
	RelatedPrNumber *int64
}

func (ghi *GithubIssueNotification) SendNotificationForProject(projectName string, repoFullName string, plan string) error {
	log.Printf("Info: Sending drift notification regarding project: %v", projectName)
	title := fmt.Sprintf("Drift detected in project: %v", projectName)
	message := fmt.Sprintf(":bangbang: Drift detected in digger project %v details below: \n\n```\n%v\n```", projectName, plan)

	// issue bodies share the comment size limit; an issue cannot be split
	// into a chain, so cap to a single chunk: SplitComment truncates the
	// head, keeps the tail with the plan summary and closes any markdown
	// structure left open at the truncation point
	message = reporting.SplitComment(message, reporting.CommentMaxSize(*ghi.GithubService), 1)[0]

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
}

func (ghi *GithubIssueNotification) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	return nil
}

func (ghi *GithubIssueNotification) Flush() error {
	return nil
}
