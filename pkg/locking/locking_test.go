package locking

import (
	"digger/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	mockDynamoDB := utils.MockLock{make(map[string]int)}
	mockPrManager := utils.MockGithubPullrequestManager{}
	pl := PullRequestLock{
		InternalLock:     &mockDynamoDB,
		CIService:        &mockPrManager,
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
		ProjectName:      "a",
		ProjectNamespace: "",
		PrNumber:         2,
	}
	state2, err2 := pl2.Lock()
	assert.False(t, state2)
	// No error because the lock was not aquired
	assert.NoError(t, err2)
}
