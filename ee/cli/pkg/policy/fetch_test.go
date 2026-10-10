package policy

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, p string, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(contents), 0o644))
}

func TestResolvePolicyFileOrder(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "policies", "access.rego"), "org")
	writeFile(t, filepath.Join(base, "policies", "repo", "access.rego"), "repo")
	writeFile(t, filepath.Join(base, "policies", "repo", "proj", "access.rego"), "project")
	writeFile(t, filepath.Join(base, "dev", "access.rego"), "dev")
	writeFile(t, filepath.Join(base, "dev", "vpc", "access.rego"), "dev/vpc")

	got, err := resolvePolicyFile(base, "repo", "proj", "dev/vpc/subnets", "access.rego")
	require.NoError(t, err)
	assert.Equal(t, "dev/vpc", got, "deepest project dir prefix wins")

	got, err = resolvePolicyFile(base, "repo", "proj", "prod/vpc", "access.rego")
	require.NoError(t, err)
	assert.Equal(t, "project", got, "falls back to policies/<repo>/<project>")

	got, err = resolvePolicyFile(base, "repo", "other", "prod/vpc", "access.rego")
	require.NoError(t, err)
	assert.Equal(t, "repo", got)

	got, err = resolvePolicyFile(base, "other-repo", "other", "prod/vpc", "access.rego")
	require.NoError(t, err)
	assert.Equal(t, "org", got)

	got, err = resolvePolicyFile(base, "repo", "proj", "dev/vpc", "plan.rego")
	require.NoError(t, err)
	assert.Equal(t, "", got, "missing file resolves to empty, not error")
}

func TestManagementRepoFetchedOncePerProcess(t *testing.T) {
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
	writeFile(t, filepath.Join(dir, "policies", "access.rego"), "package digger\nallow = false\n")
	writeFile(t, filepath.Join(dir, "dev", "main.tf"), "resource {}")
	run("add", ".")
	run("commit", "-q", "-m", "init")

	p := DiggerRepoPolicyProvider{ManagementRepoUrl: "file://" + dir, GitTimeout: 30 * time.Second}

	first, err := p.managementRepoDir()
	require.NoError(t, err)
	defer os.RemoveAll(first)
	second, err := p.managementRepoDir()
	require.NoError(t, err)
	assert.Equal(t, first, second, "second lookup reuses the checkout")

	access, err := p.GetAccessPolicy("org", "repo", "proj", "dev")
	require.NoError(t, err)
	assert.Equal(t, "package digger\nallow = false\n", access)

	plan, err := p.GetPlanPolicy("org", "repo", "proj", "dev")
	require.NoError(t, err)
	assert.Equal(t, "", plan)

	assert.NoFileExists(t, filepath.Join(first, "dev", "main.tf"), "non-policy files are not checked out")
}
