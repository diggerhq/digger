package orchestrator

type PullRequestService interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string) (int64, error)
	ListIssues() ([]*Issue, error)
	PublishIssue(title string, body string) (int64, error)
	EditComment(prNumber int, id interface{}, comment string) error
	GetComments(prNumber int) ([]Comment, error)
	GetApprovals(prNumber int) ([]string, error)
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
	GetBranchName(prNumber int) (string, error)
	SetOutput(prNumber int, key string, value string) error
}

type OrgService interface {
	GetUserTeams(organisation string, user string) ([]string, error)
}

type Issue struct {
	ID    int64
	Title string
	Body  string
}

type Comment struct {
	Id   interface{}
	Body *string
	Url  string
}

type PullRequestComment interface {
	GetUrl() (string, error)
}
