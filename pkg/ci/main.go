package ci

type CIService interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string)
	SetStatus(prNumber int, status string, statusContext string) error
	GetCombinedPullRequestStatus(prNumber int) (string, error)
	MergePullRequest(prNumber int) error
	IsMergeable(prNumber int) (bool, error)
	IsClosed(prNumber int) (bool, error)
}
