package locking

import (
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	mockDynamoDB := MockLock{MapLock: make(map[string]int)}
	mockPrManager := orchestrator.MockGithubPullrequestManager{}
	reporter := reporting.MockReporter{}
	pl := PullRequestLock{
		InternalLock:     &mockDynamoDB,
		CIService:        &mockPrManager,
		Reporter:         &reporter,
		ProjectName:      "a",
		ProjectNamespace: "",
		PrNumber:         1,
	}
	state1, err1 := pl.Lock()
	assert.True(t, state1)
	assert.NoError(t, err1)

	pl2 := PullRequestLock{
		InternalLock:     &mockDynamoDB,
		CIService:        &mockPrManager,
		Reporter:         &reporter,
		ProjectName:      "a",
		ProjectNamespace: "",
		PrNumber:         2,
	}
	state2, err2 := pl2.Lock()
	assert.False(t, state2)
	assert.Error(t, err2)
}

func TestGetLock(t *testing.T) {
	// TODO: implement this test
	lock, err := GetLock()
	if err != nil {
		print(err)
	}
	print(lock)
}
