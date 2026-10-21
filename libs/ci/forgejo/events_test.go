package forgejo

import (
	"fmt"
	"testing"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const repoJson = `"repository":{"full_name":"org/repo","default_branch":"main"},"sender":{"login":"alice"}`

func pullRequestPayload(action string, merged bool, baseRef string) []byte {
	return []byte(fmt.Sprintf(`{"action":%q,"number":7,"pull_request":{"number":7,"merged":%t,"head":{"ref":"feature","sha":"abc"},"base":{"ref":%q,"sha":"def"}},%s}`, action, merged, baseRef, repoJson))
}

func commentPayload(action string, body string, onPullRequest bool) []byte {
	pullRequest := "null"
	if onPullRequest {
		pullRequest = `{"merged":false}`
	}
	return []byte(fmt.Sprintf(`{"action":%q,"issue":{"number":7,"pull_request":%s},"comment":{"id":1,"body":%q},%s}`, action, pullRequest, body, repoJson))
}

func testConfig() digger_config.DiggerConfig {
	return digger_config.DiggerConfig{
		Projects: []digger_config.Project{
			{Name: "dev", Dir: "dev", Workflow: "default"},
			{Name: "prod", Dir: "prod", Workflow: "default"},
		},
		Workflows: map[string]digger_config.Workflow{
			"default": {Configuration: &digger_config.WorkflowConfiguration{
				OnPullRequestPushed: []string{"digger plan"},
				OnPullRequestClosed: []string{"digger unlock"},
				OnCommitToDefault:   []string{"digger unlock", "digger apply"},
			}},
		},
	}
}

func TestParseEvent(t *testing.T) {
	event, err := ParseEvent("pull_request", pullRequestPayload("opened", false, "main"))
	require.NoError(t, err)
	assert.Equal(t, "opened", event.(PullRequestEvent).Action)

	event, err = ParseEvent("pull_request_target", pullRequestPayload("synchronized", false, "main"))
	require.NoError(t, err)
	assert.Equal(t, "synchronized", event.(PullRequestEvent).Action)

	event, err = ParseEvent("issue_comment", commentPayload("created", "digger plan", true))
	require.NoError(t, err)
	assert.Equal(t, "digger plan", event.(IssueCommentEvent).Comment.Body)

	_, err = ParseEvent("issue_comment", commentPayload("created", "digger plan", false))
	assert.ErrorIs(t, err, ErrIgnoredEvent)

	_, err = ParseEvent("issue_comment", commentPayload("edited", "digger plan", true))
	assert.ErrorIs(t, err, ErrIgnoredEvent)

	_, err = ParseEvent("push", []byte(`{}`))
	assert.ErrorContains(t, err, "unsupported event type")

	_, err = ParseEvent("pull_request", []byte(`{"action":"opened"}`))
	assert.ErrorContains(t, err, "missing")
}

func TestProcessEvent(t *testing.T) {
	config := testConfig()
	prService := ci.MockPullRequestManager{ChangedFiles: []string{"dev/main.tf"}}

	event, _ := ParseEvent("pull_request", pullRequestPayload("opened", false, "main"))
	impacted, requested, prNumber, err := ProcessEvent(event, &config, prService)
	require.NoError(t, err)
	assert.Equal(t, 7, prNumber)
	assert.Nil(t, requested)
	require.Len(t, impacted, 1)
	assert.Equal(t, "dev", impacted[0].Name)

	event, _ = ParseEvent("issue_comment", commentPayload("created", "digger apply -p dev", true))
	_, requested, _, err = ProcessEvent(event, &config, prService)
	require.NoError(t, err)
	require.NotNil(t, requested)
	assert.Equal(t, "dev", requested.Name)

	event, _ = ParseEvent("issue_comment", commentPayload("created", "digger apply -p prod", true))
	_, _, _, err = ProcessEvent(event, &config, prService)
	assert.ErrorContains(t, err, "not found in modified projects")
}

func TestIsHelpComment(t *testing.T) {
	event, _ := ParseEvent("issue_comment", commentPayload("created", "digger help", true))
	assert.True(t, IsHelpComment(event))
	event, _ = ParseEvent("issue_comment", commentPayload("created", "digger plan", true))
	assert.False(t, IsHelpComment(event))
	event, _ = ParseEvent("pull_request", pullRequestPayload("opened", false, "main"))
	assert.False(t, IsHelpComment(event))
}

func TestConvertPullRequestEventToJobs(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		merged   bool
		baseRef  string
		commands []string
	}{
		{"opened", "opened", false, "main", []string{"digger plan"}},
		{"synchronized", "synchronized", false, "main", []string{"digger plan"}},
		{"reopened", "reopened", false, "main", []string{"digger plan"}},
		{"closed without merge", "closed", false, "main", []string{"digger unlock"}},
		{"merged into default branch", "closed", true, "main", []string{"digger unlock", "digger apply"}},
		{"merged into other branch", "closed", true, "release", []string{"digger unlock"}},
		{"edited", "edited", false, "main", nil},
	}
	config := testConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseEvent("pull_request", pullRequestPayload(tt.action, tt.merged, tt.baseRef))
			require.NoError(t, err)
			jobs, err := ConvertPullRequestEventToJobs(event.(PullRequestEvent), config.Projects[:1], config)
			require.NoError(t, err)
			if tt.commands == nil {
				assert.Empty(t, jobs)
				return
			}
			require.Len(t, jobs, 1)
			job := jobs[0]
			assert.Equal(t, tt.commands, job.Commands)
			assert.Equal(t, "dev", job.ProjectName)
			assert.Equal(t, "org/repo", job.Namespace)
			assert.Equal(t, "alice", job.RequestedBy)
			assert.Equal(t, "pull_request", job.EventName)
			assert.Equal(t, 7, *job.PullRequestNumber)
			assert.Equal(t, "feature", job.RunEnvVars["PR_BRANCH"])
			assert.Equal(t, "main", job.RunEnvVars["DEFAULT_BRANCH"])
		})
	}
}

func TestConvertPullRequestEventToJobsUnknownWorkflow(t *testing.T) {
	config := testConfig()
	project := digger_config.Project{Name: "x", Dir: "x", Workflow: "missing"}
	event, _ := ParseEvent("pull_request", pullRequestPayload("opened", false, "main"))
	_, err := ConvertPullRequestEventToJobs(event.(PullRequestEvent), []digger_config.Project{project}, config)
	assert.ErrorContains(t, err, "failed to find workflow config")
}
