package github

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMergeGroupPhasesDoesNotApplyAfterPlanFailure(t *testing.T) {
	planJobs := []scheduler.Job{
		{ProjectName: "network", Commands: []string{"digger plan"}},
		{ProjectName: "database", Commands: []string{"digger plan"}},
	}
	applyJobs := []scheduler.Job{
		{ProjectName: "network", Commands: []string{"digger apply"}},
		{ProjectName: "database", Commands: []string{"digger apply"}},
	}
	calls := 0
	runner := func(jobs []scheduler.Job) (bool, bool, error) {
		calls++
		for _, job := range jobs {
			assert.Equal(t, []string{"digger plan"}, job.Commands)
		}
		return false, false, nil
	}

	err := runMergeGroupPhases(planJobs, applyJobs, runner)

	require.ErrorContains(t, err, "plan phase failed")
	assert.Equal(t, 1, calls)
}

func TestRunMergeGroupPhasesPlansAllProjectsBeforeApply(t *testing.T) {
	planJobs := []scheduler.Job{
		{ProjectName: "network", Commands: []string{"digger plan"}},
		{ProjectName: "database", Commands: []string{"digger plan"}},
	}
	applyJobs := []scheduler.Job{
		{ProjectName: "network", Commands: []string{"digger apply"}},
		{ProjectName: "database", Commands: []string{"digger apply"}},
	}
	phaseCommands := make([][]string, 0, 2)
	runner := func(jobs []scheduler.Job) (bool, bool, error) {
		commands := make([]string, 0, len(jobs))
		for _, job := range jobs {
			commands = append(commands, job.Commands[0])
		}
		phaseCommands = append(phaseCommands, commands)
		return true, jobs[0].Commands[0] == "digger apply", nil
	}

	err := runMergeGroupPhases(planJobs, applyJobs, runner)

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"digger plan", "digger plan"}, {"digger apply", "digger apply"}}, phaseCommands)
}

func TestVerifyMergeGroupHead(t *testing.T) {
	output, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	head := strings.TrimSpace(string(output))

	require.NoError(t, verifyMergeGroupHead(".", head))
	require.ErrorContains(t, verifyMergeGroupHead(".", "not-the-head"), "does not match")
}
