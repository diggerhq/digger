package comment_updater

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"log"
	"strings"
)

type AdvancedCommentUpdater struct {
}

func DriftSummaryString(projectName string, issuesMap *map[string]*orchestrator.Issue) string {
	driftStatusForProject := (*issuesMap)[projectName]
	if driftStatusForProject == nil {
		return ""
	}

	return fmt.Sprintf("[drift: #%v]", driftStatusForProject.ID)
}

func (a AdvancedCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error {

	issuesMap, err := getDriftStatusesFromPRIssues(jobs, prService)
	if err != nil {
		return fmt.Errorf("error while fetching drift status: %v", err)
	}

	message := ":construction_worker: Jobs status:\n\n"
	for _, job := range jobs {
		var jobSpec orchestrator.JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job, %v", err)
			return fmt.Errorf("Failed to unmarshall serialized job: %v", err)
		}
		isPlan := jobSpec.IsPlan()

		message = message + fmt.Sprintf("<!-- PROJECTHOLDER %v -->\n", job.ProjectName)
		message = message + fmt.Sprintf("%v **%v** <a href='%v'>%v</a>%v %v\n", job.Status.ToEmoji(), jobSpec.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), job.ResourcesSummaryString(isPlan), DriftSummaryString(job.ProjectName, issuesMap))
		message = message + fmt.Sprintf("<!-- PROJECTHOLDEREND %v -->\n", job.ProjectName)
	}

	prService.EditComment(prNumber, prCommentId, message)
	return nil
}

func getDriftStatusesFromPRIssues(jobs []scheduler.SerializedJob, prService orchestrator.PullRequestService) (*map[string]*orchestrator.Issue, error) {
	issues, err := prService.ListIssues()
	if err != nil {
		return nil, fmt.Errorf("failed to list issues from SCM: %v", err)
	}
	issuesMap := make(map[string]*orchestrator.Issue)
	var issueLinked *orchestrator.Issue
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
