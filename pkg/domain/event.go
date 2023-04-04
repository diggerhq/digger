package domain

type Type int

const (
	PullRequestCreated Type = iota
	PullRequestModified
	PullRequestMerged
	PullRequestClosed
	CommentAdded
)

type Action int

const (
	Lock Action = iota
	Unlock
	Plan
	Apply
)

type Event struct {
	Type      Type
	PRDetails PRDetails
	Projects  []Project
}

type ParsedEvent struct {
	PRDetails       PRDetails
	ProjectsInScope []ProjectCommand
}
