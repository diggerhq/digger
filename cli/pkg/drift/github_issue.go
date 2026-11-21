package drift

import (
    "fmt"
    core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
    orchestrator "github.com/diggerhq/digger/libs/ci"
    "github.com/samber/lo"
    "log"
)

type GithubIssueNotification struct {
    GithubService   *orchestrator.PullRequestService
    RelatedPrNumber *int64
}

func (ghi *GithubIssueNotification) SendNotificationForProject(projectName string, repoFullName string, plan string, lastChange *core_drift.LastChange) error {
    log.Printf("Info: Sending drift notification regarding project: %v", projectName)
    title := fmt.Sprintf("Drift detected in project: %v", projectName)
    lastChangeLine := ""
    if lastChange != nil {
        lastChangeLine = fmt.Sprintf("\n\nLast change by **%v** (`%v`, %v)", lastChange.Author, lastChange.Commit, lastChange.When)
    }
    // truncate the plan itself (not the final message) so the closing code
    // fence stays intact; 64000 leaves room for the header within GitHub's
    // 65536-char issue body limit
    plan, truncated := TruncatePlan(plan, 64000)
    if truncated {
        log.Printf("drift plan truncated for github issue, project: %v", projectName)
    }
    message := fmt.Sprintf(":bangbang: Drift detected in digger project %v%v details below: \n\n```\n%v\n```", projectName, lastChangeLine, plan)
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

