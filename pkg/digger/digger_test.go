package digger

import (
	"digger/pkg/configuration"
	"digger/pkg/reporting"
	"digger/pkg/utils"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type RunInfo struct {
	Command   string
	Params    string
	Timestamp time.Time
}

type MockCommandRunner struct {
	Commands []RunInfo
}

func (m *MockCommandRunner) Run(workDir string, shell string, commands []string) (string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"Run", workDir + " " + shell + " " + strings.Join(commands, " "), time.Now()})
	return "", "", nil
}

type MockTerraformExecutor struct {
	Commands []RunInfo
}

func (m *MockTerraformExecutor) Init(params []string, envs map[string]string) (string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"Init", strings.Join(params, " "), time.Now()})
	return "", "", nil
}

func (m *MockTerraformExecutor) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	if plan != nil {
		params = append(params, *plan)
	}
	m.Commands = append(m.Commands, RunInfo{"Apply", strings.Join(params, " "), time.Now()})
	return "", "", nil
}

func (m *MockTerraformExecutor) Plan(params []string, envs map[string]string) (bool, string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"Plan", strings.Join(params, " "), time.Now()})
	return true, "", "", nil
}

type MockPRManager struct {
	Commands []RunInfo
}

func (m *MockPRManager) GetChangedFiles(prNumber int) ([]string, error) {
	m.Commands = append(m.Commands, RunInfo{"GetChangedFiles", strconv.Itoa(prNumber), time.Now()})
	return []string{}, nil
}

func (m *MockPRManager) PublishComment(prNumber int, comment string) error {
	m.Commands = append(m.Commands, RunInfo{"PublishComment", strconv.Itoa(prNumber) + " " + comment, time.Now()})
	return nil
}

func (m *MockPRManager) SetStatus(prNumber int, status string, statusContext string) error {
	m.Commands = append(m.Commands, RunInfo{"SetStatus", strconv.Itoa(prNumber) + " " + status + " " + statusContext, time.Now()})
	return nil
}

func (m *MockPRManager) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	m.Commands = append(m.Commands, RunInfo{"GetCombinedPullRequestStatus", strconv.Itoa(prNumber), time.Now()})
	return "", nil
}

func (m *MockPRManager) MergePullRequest(prNumber int) error {
	m.Commands = append(m.Commands, RunInfo{"MergePullRequest", strconv.Itoa(prNumber), time.Now()})
	return nil
}

func (m *MockPRManager) IsMergeable(prNumber int) (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"IsMergeable", strconv.Itoa(prNumber), time.Now()})
	return true, nil
}

func (m *MockPRManager) DownloadLatestPlans(prNumber int) (string, error) {
	m.Commands = append(m.Commands, RunInfo{"DownloadLatestPlans", strconv.Itoa(prNumber), time.Now()})
	return "plan", nil
}

func (m *MockPRManager) IsClosed(prNumber int) (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"IsClosed", strconv.Itoa(prNumber), time.Now()})
	return false, nil
}

type MockProjectLock struct {
	Commands []RunInfo
}

func (m *MockProjectLock) Lock() (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"Lock", "", time.Now()})
	return true, nil
}

func (m *MockProjectLock) Unlock() (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"Unlock", "", time.Now()})
	return true, nil
}

func (m *MockProjectLock) ForceUnlock() error {
	m.Commands = append(m.Commands, RunInfo{"ForceUnlock", "", time.Now()})
	return nil
}

func (m *MockProjectLock) LockId() string {
	m.Commands = append(m.Commands, RunInfo{"LockId", "", time.Now()})
	return ""
}

type MockZipper struct {
	Commands []RunInfo
}

func (m *MockZipper) GetFileFromZip(zipFile string, filename string) (string, error) {
	m.Commands = append(m.Commands, RunInfo{"GetFileFromZip", zipFile + " " + filename, time.Now()})
	return "plan", nil
}

type MockPlanStorage struct {
	Commands []RunInfo
}

func (m *MockPlanStorage) StorePlan(localPlanFilePath string, storedPlanFilePath string) error {
	m.Commands = append(m.Commands, RunInfo{"StorePlan", localPlanFilePath, time.Now()})
	return nil
}

func (m *MockPlanStorage) RetrievePlan(localPlanFilePath string, storedPlanFilePath string) (*string, error) {
	m.Commands = append(m.Commands, RunInfo{"RetrievePlan", localPlanFilePath, time.Now()})
	return nil, nil
}

func (m *MockPlanStorage) DeleteStoredPlan(storedPlanFilePath string) error {
	m.Commands = append(m.Commands, RunInfo{"DeleteStoredPlan", storedPlanFilePath, time.Now()})
	return nil
}

func (m *MockPlanStorage) PlanExists(storedPlanFilePath string) (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"PlanExists", storedPlanFilePath, time.Now()})
	return false, nil
}

func TestCorrectCommandExecutionWhenApplying(t *testing.T) {

	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}
	planStorage := &MockPlanStorage{}
	reporter := &reporting.CiReporter{
		CiService: prManager,
		PrNumber:  1,
	}
	executor := DiggerExecutor{
		ApplyStage: &configuration.Stage{
			Steps: []configuration.Step{
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
		PlanStage:         &configuration.Stage{},
		CommandRunner:     commandRunner,
		TerraformExecutor: terraformExecutor,
		Reporter:          reporter,
		ProjectLock:       lock,
		PlanStorage:       planStorage,
	}

	executor.Apply()

	commandStrings := allCommandsInOrderWithParams(terraformExecutor, commandRunner, prManager, lock, planStorage)

	assert.Equal(t, []string{"RetrievePlan #.tfplan", "Lock ", "Init ", "Apply ", "LockId ", "PublishComment 1 <details>\n  <summary>Apply for ****</summary>\n\n  ```terraform\n\n  ```\n</details>", "LockId ", "Run   echo"}, commandStrings)
}

func TestCorrectCommandExecutionWhenPlanning(t *testing.T) {
	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}
	planStorage := &MockPlanStorage{}
	reporter := &reporting.CiReporter{
		CiService: prManager,
		PrNumber:  1,
	}
	executor := DiggerExecutor{
		ApplyStage: &configuration.Stage{},
		PlanStage: &configuration.Stage{
			Steps: []configuration.Step{
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
		CommandRunner:     commandRunner,
		TerraformExecutor: terraformExecutor,
		Reporter:          reporter,
		ProjectLock:       lock,
		PlanStorage:       planStorage,
	}

	executor.Plan()

	commandStrings := allCommandsInOrderWithParams(terraformExecutor, commandRunner, prManager, lock, planStorage)

	assert.Equal(t, []string{"Lock ", "Init ", "Plan -out #.tfplan", "PlanExists #.tfplan", "StorePlan #.tfplan", "LockId ", "PublishComment 1 <details>\n  <summary>Plan for ****</summary>\n\n  ```terraform\n\n  ```\n</details>", "LockId ", "Run   echo"}, commandStrings)
}

func allCommandsInOrderWithParams(terraformExecutor *MockTerraformExecutor, commandRunner *MockCommandRunner, prManager *MockPRManager, lock *MockProjectLock, planStorage *MockPlanStorage) []string {
	var commands []RunInfo
	for _, command := range terraformExecutor.Commands {
		commands = append(commands, command)
	}
	for _, command := range commandRunner.Commands {
		commands = append(commands, command)
	}
	for _, command := range prManager.Commands {
		commands = append(commands, command)
	}
	for _, command := range lock.Commands {
		commands = append(commands, command)
	}
	for _, command := range planStorage.Commands {
		commands = append(commands, command)
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Timestamp.Before(commands[j].Timestamp)
	})

	// turn commands into string slice join command and it's arguments into a string
	var commandStrings []string
	for _, command := range commands {
		commandStrings = append(commandStrings, command.Command+" "+command.Params)
	}
	return commandStrings
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
		out, err := utils.ParseWorkspace(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("ParseWorkspace(%q) = %q, want error", tt.in, out)
			}
		} else {
			if err != nil {
				t.Errorf("ParseWorkspace(%q) = %q, want %q", tt.in, err, tt.out)
			}
			if out != tt.out {
				t.Errorf("ParseWorkspace(%q) = %q, want %q", tt.in, out, tt.out)
			}
		}
	}

}
