package comment_updater

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"log"
	"strings"
)

type AdvancedCommentUpdater struct {
}

func DriftSummaryString(projectName string, issuesMap *map[string]*ci.Issue) string {
	driftStatusForProject := (*issuesMap)[projectName]
	if driftStatusForProject == nil {
		return ""
	}

	return fmt.Sprintf("[drift: #%v]", driftStatusForProject.ID)
}

func (a AdvancedCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {

	issuesMap, err := getDriftStatusesFromPRIssues(jobs, prService)
	if err != nil {
		return fmt.Errorf("error while fetching drift status: %v", err)
	}

	message := ":construction_worker: Jobs status:\n\n"
	for _, job := range jobs {
		var jobSpec scheduler.JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job, %v", err)
			return fmt.Errorf("Failed to unmarshall serialized job: %v", err)
		}
		isPlan := jobSpec.IsPlan()

		// Safe handling of WorkflowRunUrl pointer
		workflowUrl := "#"
		if job.WorkflowRunUrl != nil {
			workflowUrl = *job.WorkflowRunUrl
		}

		message = message + fmt.Sprintf("<!-- PROJECTHOLDER %s -->\n", strings.ReplaceAll(job.ProjectName, "%", "%%"))
		message = message + fmt.Sprintf("%s **%s** <a href='%s'>%s</a>%s %s\n",
			job.Status.ToEmoji(),
			strings.ReplaceAll(jobSpec.ProjectName, "%", "%%"),
			strings.ReplaceAll(workflowUrl, "%", "%%"),
			strings.ReplaceAll(job.Status.ToString(), "%", "%%"),
			strings.ReplaceAll(job.ResourcesSummaryString(isPlan), "%", "%%"),
			strings.ReplaceAll(DriftSummaryString(job.ProjectName, issuesMap), "%", "%%"))
		message = message + fmt.Sprintf("<!-- PROJECTHOLDEREND %s -->\n", strings.ReplaceAll(job.ProjectName, "%", "%%"))
	}

	prService.EditComment(prNumber, prCommentId, message)
	return nil
}

func getDriftStatusesFromPRIssues(jobs []scheduler.SerializedJob, prService ci.PullRequestService) (*map[string]*ci.Issue, error) {
	issues, err := prService.ListIssues()
	if err != nil {
		return nil, fmt.Errorf("failed to list issues from SCM: %v", err)
	}
	issuesMap := make(map[string]*ci.Issue)
	var issueLinked *ci.Issue
	for _, job := range jobs {
		issueLinked = nil
		for _, issue := range issues {
			if strings.Contains(strings.ToLower(issue.Title), job.ProjectName) &&
				strings.Contains(strings.ToLower(issue.Title), "drift") {
				issueLinked = issue
				break
			}
		}
		issuesMap[job.ProjectName] = issueLinked
	}
	return &issuesMap, nil
}
