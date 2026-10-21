package generic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendToEnvFile(t *testing.T) {
	t.Setenv("GITHUB_ENV", "")
	assert.ErrorContains(t, AppendToEnvFile("A", "1"), "GITHUB_ENV not set")

	envFile := filepath.Join(t.TempDir(), "env")
	require.NoError(t, os.WriteFile(envFile, nil, 0644))
	t.Setenv("GITHUB_ENV", envFile)
	require.NoError(t, AppendToEnvFile("A", "1"))
	require.NoError(t, AppendToEnvFile("B", "2"))
	content, err := os.ReadFile(envFile)
	require.NoError(t, err)
	assert.Equal(t, "A=1\nB=2\n", string(content))
}

func TestIsForgejoActions(t *testing.T) {
	t.Setenv("FORGEJO_ACTIONS", "")
	t.Setenv("GITEA_ACTIONS", "")
	assert.False(t, IsForgejoActions())
	t.Setenv("GITEA_ACTIONS", "true")
	assert.True(t, IsForgejoActions())
	t.Setenv("GITEA_ACTIONS", "")
	t.Setenv("FORGEJO_ACTIONS", "true")
	assert.True(t, IsForgejoActions())
}
