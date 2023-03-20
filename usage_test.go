package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

// util function for testing of send usage record
func TestSendingUsageRecord(t *testing.T) {
	if os.Getenv("MANUAL_TEST") != "" {
		t.Skip("Skipping not finished test")
	}
	err := sendUsageRecord("repoOwner", "testEvent", "testing")
	assert.Nil(t, err)
}
