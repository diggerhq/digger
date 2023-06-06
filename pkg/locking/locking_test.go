package locking

import (
	"digger/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	mockDynamoDB := utils.MockLock{make(map[string]int)}
	mockPrManager := utils.MockGithubPullrequestManager{}
	pl := ProjectLockImpl{
		InternalLock:     &mockDynamoDB,
		CIService:        &mockPrManager,
		ProjectName:      "a",
		ProjectNamespace: "",
	}
	state1, err1 := pl.Lock(1)
	assert.True(t, state1)
	assert.NoError(t, err1)
	state2, err2 := pl.Lock(2)
	assert.False(t, state2)
	// No error because the lock was not aquired
	assert.NoError(t, err2)
}
