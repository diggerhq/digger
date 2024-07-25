package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	url := "http://gitlab.com/mike/dispora.git"
	repoName, _ := ExtractRepoName(url)
	assert.Equal(t, "mike/dispora", repoName)

	url = "http://git.mydomain.com/mike/dispora.git"
	repoName, _ = ExtractRepoName(url)
	assert.Equal(t, "mike/dispora", repoName)

}
