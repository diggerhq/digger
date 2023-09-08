package utils

import orchestrator "github.com/diggerhq/lib-orchestrator"

type MockReporter struct {
	commands []string
}

func (mockReporter *MockReporter) Report(report string, formatter func(string) string) error {
	mockReporter.commands = append(mockReporter.commands, "Report")
	return nil
}

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

func (mockGithubPullrequestManager *MockGithubPullrequestManager) PublishComment(prNumber int, comment string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "PublishComment")
	return nil
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

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetComments")
	return []orchestrator.Comment{}, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) EditComment(commentId interface{}, comment string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "EditComment")
	return nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetBranchName(prNumber int) (string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetBranchName")
	return "", nil
}
