package utils

import (
	"github.com/diggerhq/digger/libs/orchestrator"
)

type MockReporter struct {
	commands []string
}

func (mockReporter *MockReporter) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	mockReporter.commands = append(mockReporter.commands, "Report")
	return "", "", nil
}

func (mockReporter *MockReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (mockReporter *MockReporter) Suppress() error {
	return nil
}

func (mockReporter *MockReporter) SupportsMarkdown() bool {
	mockReporter.commands = append(mockReporter.commands, "SupportsMarkdown")
	return false
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

func (mockGithubPullrequestManager *MockGithubPullrequestManager) PublishComment(prNumber int, comment string) (int64, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "PublishComment")
	return 0, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) ListIssues() ([]*orchestrator.Issue, error) {
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

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetComments")
	return []orchestrator.Comment{}, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetApprovals(prNumber int) ([]string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetApprovals")
	return []string{}, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) EditComment(prNumber int, commentId interface{}, comment string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "EditComment")
	return nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetBranchName(prNumber int) (string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetBranchName")
	return "", nil
}

func (mockGithubPullrequestManager MockGithubPullrequestManager) SetOutput(prNumber int, key string, value string) error {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "SetOutput")
	return nil
}
