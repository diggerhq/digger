package utils

import (
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func TestGithubCloneWithInvalidTokenThrowsErr(t *testing.T) {
	f := func(d string) {}
	err := CloneGitRepoAndDoAction("https://github.com/diggerhq/private-repo", "main", "invalid-token", f)
	assert.NotNil(t, err)
}

func TestGithubCloneWithPublicRepoThrowsNoError(t *testing.T) {
	token := os.Getenv("GITHUB_PAT_TOKEN")
	f := func(d string) {}
	err := CloneGitRepoAndDoAction("https://github.com/diggerhq/digger", "develop", token, f)
	assert.Nil(t, err)
}

func TestGithubCloneWithInvalidBranchThrowsError(t *testing.T) {
	token := os.Getenv("GITHUB_PAT_TOKEN")
	f := func(d string) {}
	err := CloneGitRepoAndDoAction("https://github.com/diggerhq/digger", "not-a-branch", token, f)
	assert.NotNil(t, err)
}
