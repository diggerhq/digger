package main

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
