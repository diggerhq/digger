package gitlab

import (
	"os"
	"path"
	"testing"

	ci_github "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/stretchr/testify/assert"
	gitlabapi "github.com/xanzy/go-gitlab"
)

func TestProcessGitlabPullRequestEventDoesNotTraverseAcrossTargetBranchBoundaries(t *testing.T) {
	tempDir := t.TempDir()

	diggerCfg := `
dependency_configuration:
  mode: hard
projects:
- name: consumer
  dir: env/dev
- name: release_only_downstream
  dir: env/release
  branch: release
  depends_on:
    - consumer
- name: main_only_after_release
  dir: env/final
  depends_on:
    - release_only_downstream
`

	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "env", "dev"), 0o755))
	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "env", "release"), 0o755))
	assert.NoError(t, os.MkdirAll(path.Join(tempDir, "env", "final"), 0o755))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "digger.yml"), []byte(diggerCfg), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "env", "dev", "main.tf"), []byte(`resource "null_resource" "consumer" {}`), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "env", "release", "main.tf"), []byte(`resource "null_resource" "release" {}`), 0o644))
	assert.NoError(t, os.WriteFile(path.Join(tempDir, "env", "final", "main.tf"), []byte(`resource "null_resource" "final" {}`), 0o644))

	config, _, dependencyGraph, _, err := digger_config.LoadDiggerConfig(tempDir, true, nil, nil)
	assert.NoError(t, err)

	payload := &gitlabapi.MergeEvent{}
	payload.Project.DefaultBranch = "main"
	payload.ObjectAttributes.IID = 1
	payload.ObjectAttributes.TargetBranch = "main"
	payload.ObjectAttributes.Action = "open"

	ciService := ci_github.MockCiService{
		ChangedFilesPerPr: map[int][]string{
			1: {"env/dev/main.tf"},
		},
	}

	impactedProjects, _, _, err := ProcessGitlabPullRequestEvent(payload, config, dependencyGraph, ciService)
	assert.NoError(t, err)
	assert.Len(t, impactedProjects, 1)
	assert.Equal(t, "consumer", impactedProjects[0].Name)
}
