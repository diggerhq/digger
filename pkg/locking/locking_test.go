package locking

import (
	"digger/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	mockDynamoDB := utils.MockLock{make(map[string]int)}
	mockPrManager := utils.MockGithubPullrequestManager{}
	reporter := utils.MockReporter{}
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
	// No error because the lock was not acquired
	assert.NoError(t, err2)
}

func TestGetLock(t *testing.T) {
	lock, err := GetLock()
	if err != nil {
		print(err)

	}
	print(lock)

}
