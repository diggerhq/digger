package models

import "github.com/google/go-github/v54/github"

client := github.NewClient(nil)

type IssueCommentWebhookEvent struct {
	// Action is the action that was performed on the comment.
	// Possible values are: "created", "edited", "deleted".
	Action  *string       `json:"action,omitempty"`
	Issue   *Issue        `json:"issue,omitempty"`
	Comment *IssueComment `json:"comment,omitempty"`

	// The following fields are only populated by Webhook events.
	Changes      *EditChange   `json:"changes,omitempty"`
	Repo         *Repository   `json:"repository,omitempty"`
	Sender       *User         `json:"sender,omitempty"`
	Installation *Installation `json:"installation,omitempty"`

	// The following field is only present when the webhook is triggered on
	// a repository belonging to an organization.
	Organization *Organization `json:"organization,omitempty"`
}

type PullRequestWebhookEvent struct {
	// Action is the action that was performed. Possible values are:
	// "assigned", "unassigned", "review_requested", "review_request_removed", "labeled", "unlabeled",
	// "opened", "edited", "closed", "ready_for_review", "locked", "unlocked", or "reopened".
	// If the action is "closed" and the "merged" key is "false", the pull request was closed with unmerged commits.
	// If the action is "closed" and the "merged" key is "true", the pull request was merged.
	// While webhooks are also triggered when a pull request is synchronized, Events API timelines
	// don't include pull request events with the "synchronize" action.
	Action      *string      `json:"action,omitempty"`
	Assignee    *User        `json:"assignee,omitempty"`
	Number      *int         `json:"number,omitempty"`
	PullRequest *PullRequest `json:"pull_request,omitempty"`

	// The following fields are only populated by Webhook events.
	Changes *EditChange `json:"changes,omitempty"`
	// RequestedReviewer is populated in "review_requested", "review_request_removed" event deliveries.
	// A request affecting multiple reviewers at once is split into multiple
	// such event deliveries, each with a single, different RequestedReviewer.
	RequestedReviewer *User `json:"requested_reviewer,omitempty"`
	// In the event that a team is requested instead of a user, "requested_team" gets sent in place of
	// "requested_user" with the same delivery behavior.
	RequestedTeam *Team         `json:"requested_team,omitempty"`
	Repo          *Repository   `json:"repository,omitempty"`
	Sender        *User         `json:"sender,omitempty"`
	Installation  *Installation `json:"installation,omitempty"`
	Label         *Label        `json:"label,omitempty"` // Populated in "labeled" event deliveries.

	// The following field is only present when the webhook is triggered on
	// a repository belonging to an organization.
	Organization *Organization `json:"organization,omitempty"`

	// The following fields are only populated when the Action is "synchronize".
	Before *string `json:"before,omitempty"`
	After  *string `json:"after,omitempty"`
}

type EditChange struct {
	Title *EditTitle `json:"title,omitempty"`
	Body  *EditBody  `json:"body,omitempty"`
	Base  *EditBase  `json:"base,omitempty"`
	Repo  *EditRepo  `json:"repository,omitempty"`
	Owner *EditOwner `json:"owner,omitempty"`
}

type EditTitle struct {
	From *string `json:"from,omitempty"`
}

type EditBody struct {
	From *string `json:"from,omitempty"`
}

type EditBase struct {
	Ref *EditRef `json:"ref,omitempty"`
	SHA *EditSHA `json:"sha,omitempty"`
}

type EditRef struct {
	From *string `json:"from,omitempty"`
}

type EditRepo struct {
	Name *RepoName `json:"name,omitempty"`
}

type EditOwner struct {
	OwnerInfo *OwnerInfo `json:"from,omitempty"`
}
