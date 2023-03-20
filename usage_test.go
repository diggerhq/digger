package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// util function for testing of send usage record
func SendingUsageRecordTest(t *testing.T) {
	err := sendUsageRecord("repoOwner", "testEvent", "testing")
	assert.Nil(t, err)
}
