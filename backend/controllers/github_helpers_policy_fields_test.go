package controllers

import (
	"testing"

	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
)

func TestPopulatePolicyFieldsForJobsSkippedWhenDisabled(t *testing.T) {
	t.Setenv("DIGGER_DISABLE_POLICY_FIELDS_LOOKUP", "1")

	jobs := []scheduler.Job{{ProjectName: "project", RequestedBy: "user"}}

	// nil services: any GitHub call would panic, so returning cleanly proves no lookup happened
	err := populatePolicyFieldsForJobs(nil, nil, jobs, "org", 1)

	assert.NoError(t, err)
	assert.Nil(t, jobs[0].Teams)
	assert.Nil(t, jobs[0].Approvals)
	assert.Nil(t, jobs[0].ApprovalTeams)
}
