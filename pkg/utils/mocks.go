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
	MapLock map[string]int
}

func (lock *MockLock) Lock(timeout int, transactionId int, resource string) (bool, error) {
	if lock.MapLock == nil {
		lock.MapLock = make(map[string]int)
	}
	lock.MapLock[resource] = transactionId
	return true, nil
}

func (lock *MockLock) Unlock(resource string) (bool, error) {
	delete(lock.MapLock, resource)
	return true, nil
}

func (lock *MockLock) GetLock(resource string) (*int, error) {
	result, ok := lock.MapLock[resource]
	if ok {
		return &result, nil
	}
	return nil, nil
}

type MockPullRequestManager struct {
	ChangedFiles []string
}

func (t MockPullRequestManager) GetChangedFiles(prNumber int) ([]string, error) {
	return t.ChangedFiles, nil
}
func (t MockPullRequestManager) PublishComment(prNumber int, comment string) {

}
