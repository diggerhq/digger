package comment_utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetWorkflowUrl(t *testing.T) {
	t.Setenv("GITHUB_SERVER_URL", "https://git.example")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")
	t.Setenv("GITHUB_RUN_ID", "1001")
	t.Setenv("GITHUB_RUN_NUMBER", "5")

	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("FORGEJO_ACTIONS", "")
	assert.Equal(t, "#", GetWorkflowUrl())

	t.Setenv("GITHUB_ACTIONS", "true")
	assert.Equal(t, "https://git.example/org/repo/actions/runs/1001", GetWorkflowUrl())

	t.Setenv("FORGEJO_ACTIONS", "true")
	assert.Equal(t, "https://git.example/org/repo/actions/runs/5", GetWorkflowUrl())
}
