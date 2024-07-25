package utils

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	url := "http://gitlab.com/mike/dispora.git"
	repoName, _ := ExtractCleanRepoName(url)
	assert.Equal(t, "gitlab.com/mike/dispora", repoName)

	url = "http://git.mydomain.com/mike/dispora.git"
	repoName, _ = ExtractCleanRepoName(url)
	assert.Equal(t, "git.mydomain.com/mike/dispora", repoName)
}

func TestRepoAllowList(t *testing.T) {
	os.Setenv("DIGGER_REPO_ALLOW_LIST", "gitlab.com/diggerdev/digger-demo,gitlab.com/diggerdev/alsoallowed")
	url := "http://gitlab.com/mike/dispora.git"
	allowed := IsInRepoAllowList(url)
	assert.False(t, allowed)

	url = "http://gitlab.com/diggerdev/digger-demo2.git"
	allowed = IsInRepoAllowList(url)
	assert.False(t, allowed)

	url = "http://gitlab.com/diggerdev/digger-demo.git"
	allowed = IsInRepoAllowList(url)
	assert.True(t, allowed)

}
