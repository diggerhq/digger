package digger

import (
	"github.com/diggerhq/digger/libs/ci"
	orchestrator "github.com/diggerhq/digger/libs/scheduler"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/cli/pkg/core/execution"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	configuration "github.com/diggerhq/digger/libs/digger_config"
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

func (m *MockCommandRunner) Run(workDir string, shell string, commands []string, envs map[string]string) (string, string, error) {
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
	nonEmptyTerraformPlanJson := "{\"format_version\":\"1.1\",\"terraform_version\":\"1.4.6\",\"planned_values\":{\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"sensitive_values\":{}},{\"address\":\"null_resource.testx\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"testx\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"triggers\":null},\"sensitive_values\":{}}]}},\"resource_changes\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"no-op\"],\"before\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"after\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"after_unknown\":{},\"before_sensitive\":{},\"after_sensitive\":{}}},{\"address\":\"null_resource.testx\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"testx\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"create\"],\"before\":null,\"after\":{\"triggers\":null},\"after_unknown\":{\"id\":true},\"before_sensitive\":false,\"after_sensitive\":{}}}],\"prior_state\":{\"format_version\":\"1.0\",\"terraform_version\":\"1.4.6\",\"values\":{\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"sensitive_values\":{}}]}}},\"configuration\":{\"provider_config\":{\"null\":{\"name\":\"null\",\"full_name\":\"registry.terraform.io/hashicorp/null\"}},\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_config_key\":\"null\",\"schema_version\":0},{\"address\":\"null_resource.testx\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"testx\",\"provider_config_key\":\"null\",\"schema_version\":0}]}}}\n"
	m.Commands = append(m.Commands, RunInfo{"Show", strings.Join(params, " "), time.Now()})
	return nonEmptyTerraformPlanJson, "", nil
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

func (m *MockPRManager) GetApprovals(prNumber int) ([]string, error) {
	return []string{}, nil
}

func (m *MockPRManager) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	m.Commands = append(m.Commands, RunInfo{"PublishComment", strconv.Itoa(prNumber) + " " + comment, time.Now()})
	return nil, nil
}

func (m *MockPRManager) ListIssues() ([]*ci.Issue, error) {
	m.Commands = append(m.Commands, RunInfo{"ListIssues", "", time.Now()})
	return nil, nil
}

func (m *MockPRManager) PublishIssue(title string, body string) (int64, error) {
	m.Commands = append(m.Commands, RunInfo{"PublishComment", body, time.Now()})
	return 0, nil
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

func (m *MockPRManager) GetComments(prNumber int) ([]ci.Comment, error) {
	m.Commands = append(m.Commands, RunInfo{"GetComments", strconv.Itoa(prNumber), time.Now()})
	return []ci.Comment{}, nil
}

func (m *MockPRManager) EditComment(prNumber int, id interface{}, comment string) error {
	m.Commands = append(m.Commands, RunInfo{"EditComment", strconv.Itoa(id.(int)) + " " + comment, time.Now()})
	return nil
}

func (m *MockPRManager) CreateCommentReaction(id interface{}, reaction string) error {
	m.Commands = append(m.Commands, RunInfo{"EditComment", strconv.Itoa(id.(int)) + " " + reaction, time.Now()})
	return nil
}

func (m *MockPRManager) GetBranchName(prNumber int) (string, string, error) {
	m.Commands = append(m.Commands, RunInfo{"GetBranchName", strconv.Itoa(prNumber), time.Now()})
	return "", "", nil
}

func (m *MockPRManager) SetOutput(prNumber int, key string, value string) error {
	m.Commands = append(m.Commands, RunInfo{"SetOutput", strconv.Itoa(prNumber), time.Now()})
	return nil

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

func (m *MockPlanStorage) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
	m.Commands = append(m.Commands, RunInfo{"StorePlanFile", artifactName, time.Now()})
	return nil
}

func (m *MockPlanStorage) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	m.Commands = append(m.Commands, RunInfo{"RetrievePlan", localPlanFilePath, time.Now()})
	return nil, nil
}

func (m *MockPlanStorage) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	m.Commands = append(m.Commands, RunInfo{"DeleteStoredPlan", storedPlanFilePath, time.Now()})
	return nil
}

func (m *MockPlanStorage) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	m.Commands = append(m.Commands, RunInfo{"PlanExists", storedPlanFilePath, time.Now()})
	return false, nil
}

type MockPlanPathProvider struct {
	Commands []RunInfo
}

func (m MockPlanPathProvider) ArtifactName() string {
	m.Commands = append(m.Commands, RunInfo{"ArtifactName", "", time.Now()})
	return "plan"
}

func (m MockPlanPathProvider) StoredPlanFilePath() string {
	m.Commands = append(m.Commands, RunInfo{"StoredPlanFilePath", "", time.Now()})
	return "plan"
}

func (m MockPlanPathProvider) LocalPlanFilePath() string {
	m.Commands = append(m.Commands, RunInfo{"LocalPlanFilePath", "", time.Now()})
	return "plan"
}

func TestCorrectCommandExecutionWhenApplying(t *testing.T) {

	commandRunner := &MockCommandRunner{}
	terraformExecutor := &MockTerraformExecutor{}
	prManager := &MockPRManager{}
	lock := &MockProjectLock{}
	planStorage := &MockPlanStorage{}
	reporter := &reporting.CiReporter{
		CiService:         prManager,
		PrNumber:          1,
		ReportStrategy:    &reporting.MultipleCommentsStrategy{},
		IsSupportMarkdown: true,
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

	assert.Equal(t, []string{"RetrievePlan plan", "Init ", "Apply -lock-timeout=3m", "PublishComment 1 <details ><summary>Apply output</summary>\n\n```terraform\n\n```\n</details>", "Run   echo"}, commandStrings)
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

	os.WriteFile(planPathProvider.LocalPlanFilePath(), []byte{123}, 0644)
	defer os.Remove(planPathProvider.LocalPlanFilePath())

	executor.Plan()

	commandStrings := allCommandsInOrderWithParams(terraformExecutor, commandRunner, prManager, lock, planStorage, planPathProvider)

	assert.Equal(t, []string{"Init ", "Plan -out plan -lock-timeout=3m", "Show -no-color -json plan", "StorePlanFile plan", "Run   echo"}, commandStrings)
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
