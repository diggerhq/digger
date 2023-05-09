package github

type MockGithubPullrequestManager struct {
	commands []string
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) GetChangedFiles(prNumber int) ([]string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "GetChangedFiles")
	return nil, nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) PublishComment(prNumber int, comment string) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "PublishComment")
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

func (mockGithubPullrequestManager *MockGithubPullrequestManager) IsMergeable(prNumber int) (bool, string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "IsMergeable")
	return true, "", nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) DownloadLatestPlans(prNumber int) (string, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "DownloadLatestPlans")
	return "", nil
}

func (mockGithubPullrequestManager *MockGithubPullrequestManager) IsClosed(prNumber int) (bool, error) {
	mockGithubPullrequestManager.commands = append(mockGithubPullrequestManager.commands, "IsClosed")
	return false, nil
}
