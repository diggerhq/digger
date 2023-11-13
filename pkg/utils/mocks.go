package utils

import (
	"time"

	"github.com/diggerhq/digger/libs/orchestrator"
)

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

func (lock *MockLock) Lock(transactionId int, resource string) (bool, error) {
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

type MockPolicyChecker struct {
}

func (t MockPolicyChecker) CheckAccessPolicy(ciService orchestrator.OrgService, SCMOrganisation string, SCMrepository string, projectName string, command string, requestedBy string) (bool, error) {
	return false, nil
}

func (t MockPolicyChecker) CheckPlanPolicy(projectName string, command string, requestedBy string) (bool, []string, error) {
	return false, nil, nil
}

func (t MockPolicyChecker) CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error) {
	return true, nil
}

type MockPullRequestManager struct {
	ChangedFiles []string
	Teams        []string
	Approvals    []string
}

func (t MockPullRequestManager) GetUserTeams(organisation string, user string) ([]string, error) {
	return t.Teams, nil
}

func (t MockPullRequestManager) GetChangedFiles(prNumber int) ([]string, error) {
	return t.ChangedFiles, nil
}
func (t MockPullRequestManager) PublishComment(prNumber int, comment string) error {
	return nil
}

func (t MockPullRequestManager) SetStatus(prNumber int, status string, statusContext string) error {
	return nil
}

func (t MockPullRequestManager) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	return "", nil
}

func (t MockPullRequestManager) GetApprovals(prNumber int) ([]string, error) {
	return t.Approvals, nil
}

func (t MockPullRequestManager) MergePullRequest(prNumber int) error {
	return nil
}

func (t MockPullRequestManager) IsMergeable(prNumber int) (bool, error) {
	return true, nil
}

func (t MockPullRequestManager) IsMerged(prNumber int) (bool, error) {
	return false, nil
}

func (t MockPullRequestManager) DownloadLatestPlans(prNumber int) (string, error) {
	return "", nil
}

func (t MockPullRequestManager) IsClosed(prNumber int) (bool, error) {
	return false, nil
}

func (t MockPullRequestManager) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	return []orchestrator.Comment{}, nil
}

func (t MockPullRequestManager) EditComment(prNumber int, commentId interface{}, comment string) error {
	return nil
}

func (t MockPullRequestManager) GetBranchName(prNumber int) (string, error) {
	return "", nil
}

type MockPlanStorage struct {
}

func (t MockPlanStorage) StorePlan(localPlanFilePath string, storedPlanFilePath string) error {
	return nil
}

func (t MockPlanStorage) RetrievePlan(localPlanFilePath string, storedPlanFilePath string) (*string, error) {
	return nil, nil
}

func (t MockPlanStorage) DeleteStoredPlan(storedPlanFilePath string) error {
	return nil
}

func (t MockPlanStorage) PlanExists(storedPlanFilePath string) (bool, error) {
	return false, nil
}

type MockBackendApi struct {
}

func (t MockBackendApi) ReportProject(namespace string, projectName string, configuration string) error {
	return nil
}

func (t MockBackendApi) ReportProjectRun(namespace string, projectName string, startedAt time.Time, endedAt time.Time, status string, command string, output string) error {
	return nil
}

func (t MockBackendApi) ReportProjectJobStatus(namespace string, projectName string, jobId string, status string, timestamp time.Time) error {
	return nil
}
