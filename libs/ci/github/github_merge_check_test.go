package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockedOnlyByDiggerApply_AllRequiredPassingExceptDiggerApply(t *testing.T) {
	contexts := []requiredCheckContext{
		{Name: "Validate Platform Metadata", State: "SUCCESS", IsRequired: true},
		{Name: "Wiz IaC Scanner", State: "NEUTRAL", IsRequired: true},
		{Name: "digger/plan", State: "SUCCESS", IsRequired: true},
		{Name: "digger/apply", State: "", IsRequired: true},
	}

	assert.True(t, blockedOnlyByDiggerApply(contexts))
}

func TestBlockedOnlyByDiggerApply_RequiredCheckFailing(t *testing.T) {
	contexts := []requiredCheckContext{
		{Name: "Validate Platform Metadata", State: "FAILURE", IsRequired: true},
		{Name: "digger/apply", State: "", IsRequired: true},
	}

	assert.False(t, blockedOnlyByDiggerApply(contexts))
}

func TestBlockedOnlyByDiggerApply_NonRequiredCheckFailingIsIgnored(t *testing.T) {
	// Reproduces the real scenario hit during testing: our own workflow's native
	// job-status check ("Digger") and Digger's own project-scoped status check
	// (e.g. "digger-version-test-poc/apply") both failed, but neither is a required
	// check, so they must not block apply.
	contexts := []requiredCheckContext{
		{Name: "Digger", State: "FAILURE", IsRequired: false},
		{Name: "digger-version-test-poc/apply", State: "FAILURE", IsRequired: false},
		{Name: "Wiz IaC Scanner", State: "NEUTRAL", IsRequired: true},
		{Name: "digger/apply", State: "", IsRequired: true},
	}

	assert.True(t, blockedOnlyByDiggerApply(contexts))
}

func TestBlockedOnlyByDiggerApply_SkippedAndNeutralAreAccepted(t *testing.T) {
	contexts := []requiredCheckContext{
		{Name: "Validate Platform Metadata", State: "SKIPPED", IsRequired: true},
		{Name: "Wiz IaC Scanner", State: "NEUTRAL", IsRequired: true},
		{Name: "digger/apply", State: "", IsRequired: true},
	}

	assert.True(t, blockedOnlyByDiggerApply(contexts))
}

func TestBlockedOnlyByDiggerApply_NoContexts(t *testing.T) {
	assert.True(t, blockedOnlyByDiggerApply(nil))
}
