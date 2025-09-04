package utils

import (
	"github.com/diggerhq/digger/libs/git_utils"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGithubCloneWithInvalidTokenThrowsErr(t *testing.T) {
	f := func(d string) error { return nil }
	err := git_utils.CloneGitRepoAndDoAction("https://github.com/diggerhq/private-repo", "main", "", "invalid-token", "", f)
	assert.NotNil(t, err)
}

func TestGithubCloneWithPublicRepoThrowsNoError(t *testing.T) {
	token := os.Getenv("GITHUB_PAT_TOKEN")
	f := func(d string) error { return nil }
	err := git_utils.CloneGitRepoAndDoAction("https://github.com/diggerhq/digger", "develop", "", token, "", f)
	assert.Nil(t, err)
}

func TestGithubCloneWithPrivateRepoAndValidTokenThrowsNoError(t *testing.T) {
	token := os.Getenv("GITHUB_PAT_TOKEN")
	if token == "" {
		t.Skip()
		return
	}
	f := func(d string) error { return nil }
	err := git_utils.CloneGitRepoAndDoAction("https://github.com/diggerhq/infra-gcp", "main", "", token, "", f)
	assert.Nil(t, err)
}

func TestGithubCloneWithInvalidBranchThrowsError(t *testing.T) {
	token := os.Getenv("GITHUB_PAT_TOKEN")
	f := func(d string) error { return nil }
	err := git_utils.CloneGitRepoAndDoAction("https://github.com/diggerhq/digger", "not-a-branch", "", token, "", f)
	assert.NotNil(t, err)
}
