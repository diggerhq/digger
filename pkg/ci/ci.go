package ci

type CIService interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string) error
	// SetStatus set status of specified pull/merge request, status could be: "pending", "failure", "success"
	SetStatus(prNumber int, status string, statusContext string) error
	GetCombinedPullRequestStatus(prNumber int) (string, error)
	MergePullRequest(prNumber int) error
	IsMergeable(prNumber int) (bool, error)
	IsMerged(prNumber int) (bool, error)
	IsClosed(prNumber int) (bool, error)
}
