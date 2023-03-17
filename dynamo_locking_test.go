package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDynamoDbTableLockAndUnlock(t *testing.T) {
	mockDynamoDB := MockDynamoDb{make(map[string]int)}
	mockDynamoDB.Lock(1, 1, "mylock")
	lock, err := mockDynamoDB.GetLock("mylock")
	if err == nil {
		assert.Equal(t, *lock, 1, "Expected lock ID to be 1")
	} else {
		t.Fatal("Error should not exist", err)
	}
}

func TestDynamoDbTableLockExistsThrowsError(t *testing.T) {
	mockDynamoDB := MockDynamoDb{make(map[string]int)}
	_, err := mockDynamoDB.Lock(1, 1, "mylock")
	assert.NoError(t, err)
	_, err2 := mockDynamoDB.Lock(1, 1, "mylock")
	assert.Error(t, err2)
}

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
