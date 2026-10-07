package digger

import (
	"testing"

	orchestrator "github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
)

func TestParseFailOnChangesFlag(t *testing.T) {
	tests := []struct {
		name          string
		rawCommand    string
		command       string
		failOnChanges bool
		wantErr       bool
	}{
		{
			name:       "plan without the flag is untouched",
			rawCommand: "digger plan",
			command:    "digger plan",
		},
		{
			name:          "plan with the flag",
			rawCommand:    "digger plan --fail-on-changes",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			name:          "position within the command does not matter",
			rawCommand:    "digger --fail-on-changes plan",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			name:          "surrounding whitespace",
			rawCommand:    "  digger plan --fail-on-changes  ",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			name:          "single dash spelling",
			rawCommand:    "digger plan -fail-on-changes",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			name:       "a command without the flag is returned byte for byte",
			rawCommand: "  digger plan  -d 'my  dir'  ",
			command:    "  digger plan  -d 'my  dir'  ",
		},
		{
			name:          "combined with the project flags a comment can carry",
			rawCommand:    "digger plan -p dev --fail-on-changes",
			command:       "digger plan -p dev",
			failOnChanges: true,
		},
		{
			// the comment path lowercases the command for itself, so the case is left as it was
			name:          "a capitalised command",
			rawCommand:    "Digger Plan --fail-on-changes",
			command:       "Digger Plan",
			failOnChanges: true,
		},
		{
			name:          "a capitalised flag",
			rawCommand:    "digger plan --Fail-On-Changes",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			name:          "a comment that carries the flag on its command line",
			rawCommand:    "digger plan --fail-on-changes\n\nchecking whether dev is clean",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			// the GitHub web UI sends CRLF line endings
			name:          "a CRLF comment that carries the flag on its command line",
			rawCommand:    "digger plan --fail-on-changes\r\n\r\nchecking whether dev is clean",
			command:       "digger plan",
			failOnChanges: true,
		},
		{
			name:       "a comment that only mentions the flag further down",
			rawCommand: "digger plan\n\nnote to self: do not use --fail-on-changes here",
			command:    "digger plan\n\nnote to self: do not use --fail-on-changes here",
		},
		{
			name:       "an apply comment that only mentions the flag further down",
			rawCommand: "digger apply\n\n(we can't use --fail-on-changes for this one)",
			command:    "digger apply\n\n(we can't use --fail-on-changes for this one)",
		},
		{
			name:       "flag on a capitalised apply is rejected",
			rawCommand: "Digger Apply --fail-on-changes",
			wantErr:    true,
		},
		{
			name:       "an explicit value is rejected rather than ignored",
			rawCommand: "digger plan --fail-on-changes=true",
			wantErr:    true,
		},
		{
			name:       "a command that merely starts like plan is rejected",
			rawCommand: "digger planet --fail-on-changes",
			wantErr:    true,
		},
		{
			name:       "flag on apply is rejected",
			rawCommand: "digger apply --fail-on-changes",
			wantErr:    true,
		},
		{
			name:       "flag on unlock is rejected",
			rawCommand: "digger unlock --fail-on-changes",
			wantErr:    true,
		},
		{
			name:       "non digger commands are left alone",
			rawCommand: "run echo 'hello  world'",
			command:    "run echo 'hello  world'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, failOnChanges, err := ParseFailOnChangesFlag(tt.rawCommand)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.command, command)
			assert.Equal(t, tt.failOnChanges, failOnChanges)
		})
	}
}

func TestApplyCommandFlags(t *testing.T) {
	// every job using the workflow shares this slice with the digger_config
	workflowCommands := []string{"digger plan --fail-on-changes"}
	jobs := []orchestrator.Job{
		{ProjectName: "first", Commands: workflowCommands},
		{ProjectName: "second", Commands: workflowCommands},
	}

	err := ApplyCommandFlags(jobs)

	assert.NoError(t, err)
	for _, job := range jobs {
		assert.Equal(t, []string{"digger plan"}, job.Commands, job.ProjectName)
		assert.True(t, job.FailOnChanges, job.ProjectName)
	}
	assert.Equal(t, []string{"digger plan --fail-on-changes"}, workflowCommands, "the digger_config slice must not be rewritten")
}

func TestApplyCommandFlagsWithoutTheFlag(t *testing.T) {
	jobs := []orchestrator.Job{{ProjectName: "first", Commands: []string{"run echo hello", "digger plan"}}}

	err := ApplyCommandFlags(jobs)

	assert.NoError(t, err)
	assert.Equal(t, []string{"run echo hello", "digger plan"}, jobs[0].Commands)
	assert.False(t, jobs[0].FailOnChanges)
}

func TestApplyCommandFlagsRejectsTheFlagOnApply(t *testing.T) {
	jobs := []orchestrator.Job{{ProjectName: "first", Commands: []string{"digger apply --fail-on-changes"}}}

	err := ApplyCommandFlags(jobs)

	assert.ErrorContains(t, err, "only supported on 'digger plan'")
}
