package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	mockDynamoDB := MockDynamoDb{make(map[string]int)}
	mockPrManager := MockGithubPullrequestManager{}
	pl := ProjectLockImpl{
		InternalLock: &mockDynamoDB,
		PrManager:    &mockPrManager,
		ProjectName:  "",
		RepoName:     "",
	}
	state1, err1 := pl.Lock("a", 1)
	assert.True(t, state1)
	assert.NoError(t, err1)
	state2, err2 := pl.Lock("a", 2)
	assert.False(t, state2)
	// No error because the lock was not aquired
	assert.NoError(t, err2)
}
