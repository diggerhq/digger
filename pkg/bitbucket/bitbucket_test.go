package bitbucket

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseBitbucketContext(t *testing.T) {
	contextString := `{
  "event_name": "pull_request:open",
  "repo_slug": "digger_demo",
  "author": "alexey-digger"
  
}`
	context, err := ParseBitbucketContext(contextString)
	assert.NoError(t, err)
	assert.NotNil(t, context)
	assert.Equal(t, "pull_request:open", context.EventName)
	assert.Equal(t, "digger_demo", context.RepoSlug)
	assert.Equal(t, "alexey-digger", context.Author)
}
