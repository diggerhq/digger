package forgejo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
)

var ErrIgnoredEvent = errors.New("event does not need a digger run")

type PullRequestEvent struct {
	Action      string             `json:"action"`
	PullRequest *gitea.PullRequest `json:"pull_request"`
	Repository  *gitea.Repository  `json:"repository"`
	Sender      *gitea.User        `json:"sender"`
}

type IssueCommentEvent struct {
	Action     string            `json:"action"`
	Issue      *gitea.Issue      `json:"issue"`
	Comment    *gitea.Comment    `json:"comment"`
	Repository *gitea.Repository `json:"repository"`
	Sender     *gitea.User       `json:"sender"`
}

func ParseEvent(eventName string, payload []byte) (interface{}, error) {
	switch eventName {
	case "pull_request", "pull_request_target":
		var event PullRequestEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("failed to parse pull_request event: %w", err)
		}
		if event.PullRequest == nil || event.PullRequest.Head == nil || event.PullRequest.Base == nil || event.Repository == nil || event.Sender == nil {
			return nil, fmt.Errorf("pull_request event is missing pull_request, repository or sender")
		}
		return event, nil
	case "issue_comment":
		var event IssueCommentEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("failed to parse issue_comment event: %w", err)
		}
		if event.Issue == nil || event.Comment == nil || event.Repository == nil || event.Sender == nil {
			return nil, fmt.Errorf("issue_comment event is missing issue, comment, repository or sender")
		}
		if event.Issue.PullRequest == nil {
			return nil, fmt.Errorf("%w: comment on an issue, not a pull request", ErrIgnoredEvent)
		}
		if event.Action != "created" {
			return nil, fmt.Errorf("%w: issue_comment action %q", ErrIgnoredEvent, event.Action)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", eventName)
	}
}

func ProcessEvent(event interface{}, diggerConfig *digger_config.DiggerConfig, ciService ci.PullRequestService) ([]digger_config.Project, *digger_config.Project, int, error) {
	var prNumber int
	var commentBody string
	switch e := event.(type) {
	case PullRequestEvent:
		prNumber = int(e.PullRequest.Index)
		slog.Info("processing Forgejo pull request event", "prNumber", prNumber, "action", e.Action)
	case IssueCommentEvent:
		prNumber = int(e.Issue.Index)
		commentBody = e.Comment.Body
		slog.Info("processing Forgejo issue comment event", "prNumber", prNumber, "comment", commentBody)
	default:
		return nil, nil, 0, fmt.Errorf("unsupported event type %T", event)
	}

	changedFiles, err := ciService.GetChangedFiles(prNumber)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("could not get changed files: %w", err)
	}
	impactedProjects, _ := diggerConfig.GetModifiedProjects(changedFiles)
	slog.Info("identified impacted projects", "count", len(impactedProjects), "prNumber", prNumber)

	requestedProject := scheduler.ParseProjectName(commentBody)
	if requestedProject == "" {
		return impactedProjects, nil, prNumber, nil
	}
	for _, project := range impactedProjects {
		if project.Name == requestedProject {
			return impactedProjects, &project, prNumber, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("requested project %s not found in modified projects", requestedProject)
}

func IsHelpComment(event interface{}) bool {
	commentEvent, ok := event.(IssueCommentEvent)
	return ok && strings.Contains(commentEvent.Comment.Body, "digger help")
}

func commandsForAction(event PullRequestEvent, workflow *digger_config.WorkflowConfiguration) []string {
	switch event.Action {
	case "opened", "reopened", "synchronized":
		return workflow.OnPullRequestPushed
	case "closed":
		if event.PullRequest.HasMerged && event.PullRequest.Base.Ref == event.Repository.DefaultBranch {
			return workflow.OnCommitToDefault
		}
		return workflow.OnPullRequestClosed
	default:
		return nil
	}
}

func ConvertPullRequestEventToJobs(event PullRequestEvent, impactedProjects []digger_config.Project, config digger_config.DiggerConfig) ([]scheduler.Job, error) {
	jobs := make([]scheduler.Job, 0)
	prNumber := int(event.PullRequest.Index)
	for _, project := range impactedProjects {
		workflow, ok := config.Workflows[project.Workflow]
		if !ok {
			return nil, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
		}
		if workflow.Configuration == nil {
			continue
		}
		commands := commandsForAction(event, workflow.Configuration)
		if len(commands) == 0 {
			continue
		}
		job := generic.CreateJobForProject(project, workflow, commands, "pull_request", event.Repository.FullName, event.Sender.UserName, &prNumber, event.Repository.DefaultBranch, event.PullRequest.Head.Ref, true)
		jobs = append(jobs, job)
	}
	return jobs, nil
}
