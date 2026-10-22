package comment_updater

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
)

// MockPrService implements ci.PullRequestService for testing
type MockPrService struct {
	Comments map[int]map[string]string // prNumber -> commentId -> body
	lastId   int
}

func NewMockPrService() *MockPrService {
	return &MockPrService{
		Comments: make(map[int]map[string]string),
	}
}

func (m *MockPrService) EditComment(prNumber int, id string, comment string) error {
	if m.Comments[prNumber] == nil {
		return fmt.Errorf("comment not found")
	}
	m.Comments[prNumber][id] = comment
	return nil
}

func (m *MockPrService) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	if m.Comments[prNumber] == nil {
		m.Comments[prNumber] = make(map[string]string)
	}
	m.lastId++
	id := strconv.Itoa(m.lastId)
	m.Comments[prNumber][id] = comment
	return &ci.Comment{Id: id, Body: &comment}, nil
}

func (m *MockPrService) GetChangedFiles(prNumber int) ([]string, error) { return nil, nil }
func (m *MockPrService) ListIssues() ([]*ci.Issue, error)              { return nil, nil }
func (m *MockPrService) PublishIssue(title string, body string, labels *[]string) (int64, error) {
	return 0, nil
}
func (m *MockPrService) UpdateIssue(ID int64, title string, body string) (int64, error) {
	return 0, nil
}
func (m *MockPrService) DeleteComment(id string) error                             { return nil }
func (m *MockPrService) CreateCommentReaction(id string, reaction string) error    { return nil }
func (m *MockPrService) GetComments(prNumber int) ([]ci.Comment, error)            { return nil, nil }
func (m *MockPrService) GetApprovals(prNumber int) ([]string, error)               { return nil, nil }
func (m *MockPrService) SetStatus(prNumber int, status string, ctx string) error   { return nil }
func (m *MockPrService) GetCombinedPullRequestStatus(prNumber int) (string, error) { return "", nil }
func (m *MockPrService) MergePullRequest(prNumber int, mergeStrategy string) error { return nil }
func (m *MockPrService) IsMergeable(prNumber int) (bool, error)                    { return true, nil }
func (m *MockPrService) IsMerged(prNumber int) (bool, error)                       { return false, nil }
func (m *MockPrService) IsClosed(prNumber int) (bool, error)                       { return false, nil }
func (m *MockPrService) IsDivergedFromBranch(source string, target string) (bool, error) {
	return false, nil
}
func (m *MockPrService) GetBranchName(prNumber int) (string, string, string, string, error) {
	return "", "", "", "", nil
}
func (m *MockPrService) SetOutput(prNumber int, key string, value string) error { return nil }

// helper to create a SerializedJob with the given job type
func makeSerializedJob(projectName string, jobType string) scheduler.SerializedJob {
	jobJson := scheduler.JobJson{
		JobType:     jobType,
		ProjectName: projectName,
	}
	jobBytes, _ := json.Marshal(jobJson)
	status := scheduler.DiggerJobSucceeded
	return scheduler.SerializedJob{
		DiggerJobId:  "job-1",
		Status:       status,
		ProjectName:  projectName,
		JobString:    jobBytes,
		PRCommentUrl: "#",
	}
}

func TestBasicCommentUpdater_PlanCommentIncludesApplyInstructions(t *testing.T) {
	mockPr := NewMockPrService()
	// Seed an existing comment to be updated
	mockPr.Comments[1] = map[string]string{"100": "initial"}

	updater := BasicCommentUpdater{}
	jobs := []scheduler.SerializedJob{makeSerializedJob("my-project", "plan")}

	err := updater.UpdateComment(jobs, 1, mockPr, "100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	comment := mockPr.Comments[1]["100"]
	if !strings.Contains(comment, "digger apply") {
		t.Error("plan comment should include 'digger apply' instructions")
	}
	if !strings.Contains(comment, "digger unlock") {
		t.Error("plan comment should include 'digger unlock' instructions")
	}
}

func TestBasicCommentUpdater_ApplyCommentOmitsApplyInstructions(t *testing.T) {
	mockPr := NewMockPrService()
	// Seed an existing comment to be updated
	mockPr.Comments[1] = map[string]string{"100": "initial"}

	updater := BasicCommentUpdater{}
	jobs := []scheduler.SerializedJob{makeSerializedJob("my-project", "apply")}

	err := updater.UpdateComment(jobs, 1, mockPr, "100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	comment := mockPr.Comments[1]["100"]
	if strings.Contains(comment, "digger apply") {
		t.Error("apply comment should NOT include 'digger apply' instructions")
	}
	if !strings.Contains(comment, "digger unlock") {
		t.Error("apply comment should still include 'digger unlock' instructions")
	}
}

func TestFormatExampleCommands_PlanIncludesApplyInstructions(t *testing.T) {
	result := formatExampleCommands(scheduler.DiggerCommandPlan)

	if !strings.Contains(result, "digger apply") {
		t.Error("plan command should include 'digger apply' instructions")
	}
	if !strings.Contains(result, "digger unlock") {
		t.Error("plan command should include 'digger unlock' instructions")
	}
}

func TestFormatExampleCommands_ApplyOmitsApplyInstructions(t *testing.T) {
	result := formatExampleCommands(scheduler.DiggerCommandApply)

	if strings.Contains(result, "digger apply") {
		t.Error("apply command should not include 'digger apply' instructions")
	}
	if !strings.Contains(result, "digger unlock") {
		t.Error("apply command should still include 'digger unlock' instructions")
	}
}
