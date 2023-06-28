package ci

type CIService interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string) error
	EditComment(id interface{}, comment string) error
	GetComments(prNumber int) ([]Comment, error)
	// SetStatus set status of specified pull/merge request, status could be: "pending", "failure", "success"
	SetStatus(prNumber int, status string, statusContext string) error
	GetCombinedPullRequestStatus(prNumber int) (string, error)
	MergePullRequest(prNumber int) error
	// IsMergeable is still open and ready to be merged
	IsMergeable(prNumber int) (bool, error)
	// IsMerged merged and closed
	IsMerged(prNumber int) (bool, error)
	// IsClosed closed without merging
	IsClosed(prNumber int) (bool, error)
	GetUserTeams(organisation string, user string) ([]string, error)
}

type Comment struct {
	Id   interface{}
	Body string
}
