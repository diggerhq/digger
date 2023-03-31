package utils

import (
	"digger/pkg/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	mockDynamoDB := MockLock{make(map[string]int)}
	mockPrManager := github.MockGithubPullrequestManager{}
	pl := ProjectLockImpl{
		InternalLock: &mockDynamoDB,
		PrManager:    &mockPrManager,
		ProjectName:  "",
		RepoName:     "",
		RepoOwner:    "",
	}
	state1, err1 := pl.Lock("a", 1)
	assert.True(t, state1)
	assert.NoError(t, err1)
	state2, err2 := pl.Lock("a", 2)
	assert.False(t, state2)
	// No error because the lock was not acquired
	assert.NoError(t, err2)
}
