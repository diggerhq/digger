package orchestrator

import "github.com/diggerhq/digger/libs/ci"

type MockGithubPullrequestManager struct {
	commands []string
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetUserTeams(organisation string, user string) ([]string, error) {
	return []string{}, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetChangedFiles(prNumber int) ([]string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetChangedFiles")
	return nil, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "PublishComment")
	return nil, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) ListIssues() ([]*ci.Issue, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "ListIssues")
	return nil, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) PublishIssue(title string, body string) (int64, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "PublishIssue")
	return 0, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) SetStatus(prNumber int, status string, statusContext string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "SetStatus")
	return nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetCombinedPullRequestStatus")
	return "", nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) MergePullRequest(prNumber int) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "MergePullRequest")
	return nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) IsMergeable(prNumber int) (bool, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "IsMergeable")
	return true, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) DownloadLatestPlans(prNumber int) (string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "DownloadLatestPlans")
	return "", nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) IsClosed(prNumber int) (bool, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "IsClosed")
	return false, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) IsMerged(prNumber int) (bool, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "IsClosed")
	return false, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetComments(prNumber int) ([]ci.Comment, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetComments")
	return []ci.Comment{}, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetApprovals(prNumber int) ([]string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetApprovals")
	return []string{}, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) EditComment(prNumber int, id string, comment string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "EditComment")
	return nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) CreateCommentReaction(id string, reaction string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "CreateCommentReaction")
	return nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetBranchName(prNumber int) (string, string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetBranchName")
	return "", "", nil
}

func (mockGithubPullrequestManager MockGithubPullrequestManager) SetOutput(prNumber int, key string, value string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "SetOutput")
	return nil
}
