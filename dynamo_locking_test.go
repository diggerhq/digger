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
