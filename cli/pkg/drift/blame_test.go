package drift

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitRun(t *testing.T, dir string, authorName string, authorEmail string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorName,
		"GIT_AUTHOR_EMAIL="+authorEmail,
		"GIT_COMMITTER_NAME="+authorName,
		"GIT_COMMITTER_EMAIL="+authorEmail,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
}

func TestGetLastChangeReturnsLastAuthorForProjectDir(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "Alice", "alice@example.com", "init")

	projectA := filepath.Join(repo, "projects", "a")
	projectB := filepath.Join(repo, "projects", "b")
	require.NoError(t, os.MkdirAll(projectA, 0o755))
	require.NoError(t, os.MkdirAll(projectB, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(projectA, "main.tf"), []byte("# a v1\n"), 0o644))
	gitRun(t, repo, "Alice", "alice@example.com", "add", ".")
	gitRun(t, repo, "Alice", "alice@example.com", "commit", "-m", "alice adds project a")

	require.NoError(t, os.WriteFile(filepath.Join(projectB, "main.tf"), []byte("# b v1\n"), 0o644))
	gitRun(t, repo, "Bob", "bob@example.com", "add", ".")
	gitRun(t, repo, "Bob", "bob@example.com", "commit", "-m", "bob adds project b")

	// project a was last touched by Alice even though Bob has the newest commit
	lastChangeA, err := GetLastChange(projectA)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", lastChangeA.Author)
	assert.NotEmpty(t, lastChangeA.Commit)
	assert.NotEmpty(t, lastChangeA.When)

	lastChangeB, err := GetLastChange(projectB)
	assert.NoError(t, err)
	assert.Equal(t, "Bob", lastChangeB.Author)
}

func TestGetLastChangeFailsOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()
	lastChange, err := GetLastChange(dir)
	assert.Error(t, err)
	assert.Nil(t, lastChange)
}

func TestGetLastChangeRefusesShallowClone(t *testing.T) {
	origin := t.TempDir()
	gitRun(t, origin, "Alice", "alice@example.com", "init", "--bare")

	seed := t.TempDir()
	gitRun(t, seed, "Alice", "alice@example.com", "clone", origin, "seed")
	seedRepo := filepath.Join(seed, "seed")
	require.NoError(t, os.MkdirAll(filepath.Join(seedRepo, "projects", "a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedRepo, "projects", "a", "main.tf"), []byte("# v1\n"), 0o644))
	gitRun(t, seedRepo, "Alice", "alice@example.com", "add", ".")
	gitRun(t, seedRepo, "Alice", "alice@example.com", "commit", "-m", "first")
	require.NoError(t, os.WriteFile(filepath.Join(seedRepo, "projects", "a", "main.tf"), []byte("# v2\n"), 0o644))
	gitRun(t, seedRepo, "Bob", "bob@example.com", "add", ".")
	gitRun(t, seedRepo, "Bob", "bob@example.com", "commit", "-m", "second")
	gitRun(t, seedRepo, "Bob", "bob@example.com", "push", "origin", "HEAD")

	shallowParent := t.TempDir()
	gitRun(t, shallowParent, "Carol", "carol@example.com", "clone", "--depth", "1", "file://"+origin, "shallow")
	shallowProject := filepath.Join(shallowParent, "shallow", "projects", "a")

	lastChange, err := GetLastChange(shallowProject)
	assert.Error(t, err)
	assert.Nil(t, lastChange)
	assert.Contains(t, err.Error(), "shallow clone")
}
