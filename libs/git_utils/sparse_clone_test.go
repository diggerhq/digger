package git_utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLocalRepo creates a repository on disk with a few policy files and one
// large unrelated file, and returns a file:// URL to it.
func newLocalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "-q", "-b", "main")
	run("config", "uploadpack.allowFilter", "true")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dev", "vpc"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "policies"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dev", "vpc", "access.rego"), []byte("package digger\nallow = true\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policies", "plan.rego"), []byte("package digger\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dev", "vpc", "main.tf"), make([]byte, 1<<20), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644))
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return "file://" + dir
}

func TestCloneGitRepoSparseWithTimeoutFetchesOnlyPolicyFiles(t *testing.T) {
	url := newLocalRepo(t)

	dir, err := CloneGitRepoSparseWithTimeout(url, "main", "", "", 30*time.Second, []string{"access.rego", "plan.rego", "apply.rego"})
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	assert.FileExists(t, filepath.Join(dir, "dev", "vpc", "access.rego"))
	assert.FileExists(t, filepath.Join(dir, "policies", "plan.rego"))
	assert.NoFileExists(t, filepath.Join(dir, "dev", "vpc", "main.tf"))
	assert.NoFileExists(t, filepath.Join(dir, "README.md"))

	// The large blob must not have been downloaded at all, not merely left unchecked-out.
	cmd := exec.Command("git", "cat-file", "-e", "HEAD:dev/vpc/main.tf")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	assert.Error(t, cmd.Run(), "main.tf blob should be missing from the partial clone")
}

func TestCloneGitRepoSparseWithTimeoutFallsBackOnBadBranch(t *testing.T) {
	url := newLocalRepo(t)
	_, err := CloneGitRepoSparseWithTimeout(url, "does-not-exist", "", "", 30*time.Second, []string{"access.rego"})
	assert.Error(t, err)
}
