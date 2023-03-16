package main

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
	"testing"
)

func TestProcessPullRequestCommentPlan(t *testing.T) {

	diggerConfig := &DiggerConfig{}
	eventName := "test"
	commentBody := "digger plan"
	tf := MockTerraform{}
	err := processPullRequestComment(diggerConfig, nil, eventName, &DynamoDbLock{}, &tf, 1, commentBody)
	if err != nil {
		return
	}
	assert.True(t, slices.Contains(tf.commands, "plan"))
	assert.True(t, len(tf.commands) == 1)
}

func TestProcessPullRequestCommentApply(t *testing.T) {
	diggerConfig := &DiggerConfig{}
	eventName := "test"
	commentBody := "digger apply"
	tf := MockTerraform{}
	err := processPullRequestComment(diggerConfig, nil, eventName, &DynamoDbLock{}, &tf, 1, commentBody)
	if err != nil {
		return
	}
	assert.True(t, slices.Contains(tf.commands, "apply"))
	assert.True(t, len(tf.commands) == 1)
}

func TestProcessPullRequestCommentInvalidCmd(t *testing.T) {
	diggerConfig := &DiggerConfig{}
	eventName := "test"
	commentBody := "digger digger"
	tf := MockTerraform{}
	err := processPullRequestComment(diggerConfig, nil, eventName, &DynamoDbLock{}, &tf, 1, commentBody)
	if err != nil {
		return
	}
	assert.True(t, len(tf.commands) == 0)
}
