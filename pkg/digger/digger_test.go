package digger

import (
	"digger/pkg/core/execution"
	"digger/pkg/reporting"
	"digger/pkg/utils"
	configuration "github.com/diggerhq/lib-digger-config"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	orchestrator "github.com/diggerhq/lib-orchestrator"
	"github.com/dominikbraun/graph"
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

func (m *MockTerraformExecutor) Destroy(params []string, envs map[string]string) (string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"Destroy", strings.Join(params, " "), time.Now()})
	return "", "", nil
}

func (m *MockTerraformExecutor) Show(params []string, envs map[string]string) (string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"Show", strings.Join(params, " "), time.Now()})
	return "", "", nil
}

func (m *MockTerraformExecutor) Plan(params []string, envs map[string]string) (bool, string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"Plan", strings.Join(params, " "), time.Now()})
	return true, "", "", nil
}

type MockPRManager struct {
	Commands []RunInfo
}

func (m *MockPRManager) GetUserTeams(organisation string, user string) ([]string, error) {
	return []string{}, nil
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

func (m *MockPRManager) IsMerged(prNumber int) (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"IsClosed", strconv.Itoa(prNumber), time.Now()})
	return false, nil
}

func (m *MockPRManager) IsClosed(prNumber int) (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"IsClosed", strconv.Itoa(prNumber), time.Now()})
	return false, nil
}

func (m *MockPRManager) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	m.Commands = append(m.Commands, RunInfo{"GetComments", strconv.Itoa(prNumber), time.Now()})
	return []orchestrator.Comment{}, nil
}

func (m *MockPRManager) EditComment(id interface{}, comment string) error {
	m.Commands = append(m.Commands, RunInfo{"EditComment", strconv.Itoa(id.(int)) + " " + comment, time.Now()})
	return nil
}

func (m *MockPRManager) GetBranchName(prNumber int) (string, error) {
	m.Commands = append(m.Commands, RunInfo{"GetBranchName", strconv.Itoa(prNumber), time.Now()})
	return "", nil
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

type MockPlanPathProvider struct {
	Commands []RunInfo
}

func (m MockPlanPathProvider) PlanFileName() string {
	m.Commands = append(m.Commands, RunInfo{"PlanFileName", "", time.Now()})
	return "plan"
}

func (m MockPlanPathProvider) LocalPlanFilePath() string {
	m.Commands = append(m.Commands, RunInfo{"LocalPlanFilePath", "", time.Now()})
	return "plan"
}

func (m MockPlanPathProvider) StoredPlanFilePath() string {
	m.Commands = append(m.Commands, RunInfo{"StoredPlanFilePath", "", time.Now()})
	return "plan"
}

func TestCorrectCommandExecutionWhenApplying(t *testing.T) {

	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}
	planStorage := &MockPlanStorage{}
	reporter := &reporting.CiReporter{
		CiService:      prManager,
		PrNumber:       1,
		ReportStrategy: &reporting.MultipleCommentsStrategy{},
	}
	planPathProvider := &MockPlanPathProvider{}
	executor := execution.DiggerExecutor{
		ApplyStage: &orchestrator.Stage{
			Steps: []orchestrator.Step{
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
		PlanStage:         &orchestrator.Stage{},
		CommandRunner:     commandRunner,
		TerraformExecutor: terraformExecutor,
		Reporter:          reporter,
		PlanStorage:       planStorage,
		PlanPathProvider:  planPathProvider,
	}

	executor.Apply()

	commandStrings := allCommandsInOrderWithParams(terraformExecutor, commandRunner, prManager, lock, planStorage, planPathProvider)

	assert.Equal(t, []string{"RetrievePlan plan", "Init ", "Apply -lock-timeout=3m", "PublishComment 1 <details><summary>Apply for <b>#</b></summary>\n  \n```terraform\n\n  ```\n</details>", "Run   echo"}, commandStrings)
}

func TestCorrectCommandExecutionWhenDestroying(t *testing.T) {

	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}
	planStorage := &MockPlanStorage{}
	reporter := &reporting.CiReporter{
		CiService:      prManager,
		PrNumber:       1,
		ReportStrategy: &reporting.MultipleCommentsStrategy{},
	}
	planPathProvider := &MockPlanPathProvider{}
	executor := execution.DiggerExecutor{
		ApplyStage: &orchestrator.Stage{
			Steps: []orchestrator.Step{
				{
					Action:    "init",
					ExtraArgs: nil,
					Value:     "",
				},
				{
					Action:    "destroy",
					ExtraArgs: nil,
					Value:     "",
				},
			},
		},
		PlanStage:         &orchestrator.Stage{},
		CommandRunner:     commandRunner,
		TerraformExecutor: terraformExecutor,
		Reporter:          reporter,
		PlanPathProvider:  planPathProvider,
	}

	executor.Destroy()

	commandStrings := allCommandsInOrderWithParams(terraformExecutor, commandRunner, prManager, lock, planStorage, planPathProvider)

	assert.Equal(t, []string{"Init ", "Destroy -lock-timeout=3m"}, commandStrings)
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
	planPathProvider := &MockPlanPathProvider{}

	executor := execution.DiggerExecutor{
		ApplyStage: &orchestrator.Stage{},
		PlanStage: &orchestrator.Stage{
			Steps: []orchestrator.Step{
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
		PlanStorage:       planStorage,
		PlanPathProvider:  planPathProvider,
	}

	executor.Plan()

	commandStrings := allCommandsInOrderWithParams(terraformExecutor, commandRunner, prManager, lock, planStorage, planPathProvider)

	assert.Equal(t, []string{"Init ", "Plan -out plan -lock-timeout=3m", "PlanExists plan", "StorePlan plan", "Show -no-color -json plan", "Run   echo"}, commandStrings)
}

func allCommandsInOrderWithParams(terraformExecutor *MockTerraformExecutor, commandRunner *MockCommandRunner, prManager *MockPRManager, lock *MockProjectLock, planStorage *MockPlanStorage, planPathProvider *MockPlanPathProvider) []string {
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
	for _, command := range planPathProvider.Commands {
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

func TestSortedCommandByDependency(t *testing.T) {
	//	jobs []models.Job,
	//	dependencyGraph *graph.Graph[string, string],

	jobs := []orchestrator.Job{
		{
			ProjectName: "project1",
			Commands: []string{
				"command1", "command2",
			},
		},
		{
			ProjectName: "project2",
			Commands: []string{
				"command3", "command4",
			},
		},
		{
			ProjectName: "project3",
			Commands: []string{
				"command5",
			},
		},
		{
			ProjectName: "project4",
			Commands: []string{
				"command6",
			},
		},
	}

	projectHash := func(p configuration.Project) string {
		return p.Name
	}
	dependencyGraph := graph.New(projectHash, graph.PreventCycles(), graph.Directed())

	dependencyGraph.AddVertex(configuration.Project{Name: "project1"})
	dependencyGraph.AddVertex(configuration.Project{Name: "project2"})
	dependencyGraph.AddVertex(configuration.Project{Name: "project3"})
	dependencyGraph.AddVertex(configuration.Project{Name: "project4"})

	dependencyGraph.AddEdge("project2", "project1")
	dependencyGraph.AddEdge("project3", "project2")
	dependencyGraph.AddEdge("project4", "project1")

	sortedCommands := SortedCommandsByDependency(jobs, &dependencyGraph)

	assert.Equal(t, "project3", sortedCommands[0].ProjectName)
	assert.Equal(t, "project4", sortedCommands[1].ProjectName)
	assert.Equal(t, "project2", sortedCommands[2].ProjectName)
	assert.Equal(t, "project1", sortedCommands[3].ProjectName)

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
