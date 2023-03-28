package digger

const (
	PullRequestCreated  = "pr_created"
	PullRequestClosed   = "pr_closed"
	PullRequestApproved = "pr_approved"
	PullRequestUpdated  = "pr_updated"
	CommentCreated      = "comment_created"
)

type EventType string

type Event struct {
	Type EventType
}
