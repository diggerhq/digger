package ci

type MockPullRequestManager struct {
	ChangedFiles []string
	Teams        []string
	Approvals    []string
}

func (t MockPullRequestManager) GetUserTeams(organisation string, user string) ([]string, error) {
	return t.Teams, nil
}

func (t MockPullRequestManager) GetChangedFiles(prNumber int) ([]string, error) {
	return t.ChangedFiles, nil
}
func (t MockPullRequestManager) PublishComment(prNumber int, comment string) (*Comment, error) {
	return nil, nil
}

func (t MockPullRequestManager) ListIssues() ([]*Issue, error) {
	return nil, nil
}

func (t MockPullRequestManager) PublishIssue(title string, body string) (int64, error) {
	return 0, nil
}

func (t MockPullRequestManager) SetStatus(prNumber int, status string, statusContext string) error {
	return nil
}

func (t MockPullRequestManager) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	return "", nil
}

func (t MockPullRequestManager) GetApprovals(prNumber int) ([]string, error) {
	return t.Approvals, nil
}

func (t MockPullRequestManager) MergePullRequest(prNumber int) error {
	return nil
}

func (t MockPullRequestManager) IsMergeable(prNumber int) (bool, error) {
	return true, nil
}

func (t MockPullRequestManager) IsMerged(prNumber int) (bool, error) {
	return false, nil
}

func (t MockPullRequestManager) DownloadLatestPlans(prNumber int) (string, error) {
	return "", nil
}

func (t MockPullRequestManager) IsClosed(prNumber int) (bool, error) {
	return false, nil
}

func (t MockPullRequestManager) GetComments(prNumber int) ([]Comment, error) {
	return []Comment{}, nil
}

func (t MockPullRequestManager) EditComment(prNumber int, id string, comment string) error {
	return nil
}

func (t MockPullRequestManager) CreateCommentReaction(id string, reaction string) error {
	return nil
}

func (t MockPullRequestManager) GetBranchName(prNumber int) (string, string, error) {
	return "", "", nil
}

func (t MockPullRequestManager) SetOutput(prNumber int, key string, value string) error {
	return nil
}
