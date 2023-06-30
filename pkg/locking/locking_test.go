package locking

import (
	"digger/pkg/utils"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, err2, errors.New("Project #a locked by another PR #1 (failed to acquire lock a). The locking plan must be applied or discarded before future plans can execute"))
}
