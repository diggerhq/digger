package digger

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type MockCommandRunner struct {
	Commands []string
}

func (m *MockCommandRunner) Run(command string) (string, string, error) {
	m.Commands = append(m.Commands, "Run")
	return "", "", nil
}

type MockTerraformExecutor struct {
	Commands []string
}

func (m *MockTerraformExecutor) Init(params []string) (string, string, error) {
	m.Commands = append(m.Commands, "Init")
	return "", "", nil
}

func (m *MockTerraformExecutor) Apply(params []string) (string, string, error) {
	m.Commands = append(m.Commands, "Apply")
	return "", "", nil
}

func (m *MockTerraformExecutor) Plan(params []string) (bool, string, string, error) {
	m.Commands = append(m.Commands, "Plan")
	return true, "", "", nil
}

type MockPRManager struct {
	Commands []string
}

func (m *MockPRManager) GetChangedFiles(prNumber int) ([]string, error) {
	m.Commands = append(m.Commands, "GetChangedFiles")
	return []string{}, nil
}

func (m *MockPRManager) PublishComment(prNumber int, comment string) {
	m.Commands = append(m.Commands, "PublishComment")
}

func (m *MockPRManager) SetStatus(prNumber int, status string, statusContext string) error {
	m.Commands = append(m.Commands, "SetStatus")
	return nil
}

func (m *MockPRManager) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	m.Commands = append(m.Commands, "GetCombinedPullRequestStatus")
	return "", nil
}

func (m *MockPRManager) MergePullRequest(prNumber int) error {
	m.Commands = append(m.Commands, "MergePullRequest")
	return nil
}

func (m *MockPRManager) IsMergeable(prNumber int) (bool, string, error) {
	m.Commands = append(m.Commands, "IsMergeable")
	return true, "", nil
}

type MockProjectLock struct {
	Commands []string
}

func (m *MockProjectLock) Lock(prNumber int) (bool, error) {
	m.Commands = append(m.Commands, "Lock")
	return true, nil
}

func (m *MockProjectLock) Unlock(prNumber int) (bool, error) {
	m.Commands = append(m.Commands, "Unlock")
	return true, nil
}

func (m *MockProjectLock) ForceUnlock(prNumber int) error {
	m.Commands = append(m.Commands, "ForceUnlock")
	return nil
}

func (m *MockProjectLock) LockId() string {
	m.Commands = append(m.Commands, "LockId")
	return ""
}

func TestCorrectCommandExecutionWhenApplying(t *testing.T) {

	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}

	executor := DiggerExecutor{
		applyStage: Stage{
			Steps: []Step{
				{
					Action:    "init",
					ExtraArgs: nil,
					Value:     "",
				},
				{
					Action:    "apply",
					ExtraArgs: nil,
					Value:     "",
				},
				{
					Action:    "run",
					ExtraArgs: nil,
					Value:     "echo",
				},
			},
		},
		planStage:         Stage{},
		commandRunner:     commandRunner,
		terraformExecutor: terraformExecutor,
		prManager:         prManager,
		lock:              lock,
	}

	executor.Apply(1)

	assert.Equal(t, []string{"Lock", "LockId", "Unlock", "LockId"}, lock.Commands)
	assert.Equal(t, []string{"Run"}, commandRunner.Commands)
	assert.Equal(t, []string{"Init", "Apply"}, terraformExecutor.Commands)
	assert.Equal(t, []string{"IsMergeable", "PublishComment", "PublishComment"}, prManager.Commands)
}

func TestCorrectCommandExecutionWhenPlanning(t *testing.T) {

	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}

	executor := DiggerExecutor{
		applyStage: Stage{},
		planStage: Stage{
			Steps: []Step{
				{
					Action:    "init",
					ExtraArgs: nil,
					Value:     "",
				},
				{
					Action:    "plan",
					ExtraArgs: nil,
					Value:     "",
				},
				{
					Action:    "run",
					ExtraArgs: nil,
					Value:     "echo",
				},
			},
		},
		commandRunner:     commandRunner,
		terraformExecutor: terraformExecutor,
		prManager:         prManager,
		lock:              lock,
	}

	executor.Plan(1)

	assert.Equal(t, []string{"Lock", "LockId", "LockId"}, lock.Commands)
	assert.Equal(t, []string{"Run"}, commandRunner.Commands)
	assert.Equal(t, []string{"Init", "Plan"}, terraformExecutor.Commands)
	assert.Equal(t, []string{"PublishComment", "PublishComment"}, prManager.Commands)
}

func TestParseWorkspace(t *testing.T) {
	var commentTests = []struct {
		in  string
		out string
		err bool
	}{
		{"test", "", false},
		{"test -w workspace", "workspace", false},
		{"test -w workspace -w workspace2", "", true},
		{"test -w", "", true},
	}

	for _, tt := range commentTests {
		out, err := parseWorkspace(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("parseWorkspace(%q) = %q, want error", tt.in, out)
			}
		} else {
			if err != nil {
				t.Errorf("parseWorkspace(%q) = %q, want %q", tt.in, err, tt.out)
			}
			if out != tt.out {
				t.Errorf("parseWorkspace(%q) = %q, want %q", tt.in, out, tt.out)
			}
		}
	}

}
