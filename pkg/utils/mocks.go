package utils

type MockTerraform struct {
	commands []string
}

func (tf *MockTerraform) Apply() (string, string, error) {
	tf.commands = append(tf.commands, "apply")
	return "", "", nil
}

func (tf *MockTerraform) Plan() (bool, string, string, error) {
	tf.commands = append(tf.commands, "plan")
	return true, "", "", nil
}

type MockLock struct {
}

func (lock *MockLock) Lock(timeout int, transactionId int, resource string) (bool, error) {
	return true, nil
}

func (lock *MockLock) Unlock(resource string) (bool, error) {
	return true, nil
}
func (lock *MockLock) GetLock(resource string) (*int, error) {
	i := 1
	return &i, nil
}

type MockPullRequestManager struct {
	ChangedFiles []string
}

func (t MockPullRequestManager) GetChangedFiles(prNumber int) ([]string, error) {
	return t.ChangedFiles, nil
}
func (t MockPullRequestManager) PublishComment(prNumber int, comment string) {

}
