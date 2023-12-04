package gitlab

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseGitLabContext(t *testing.T) {
	t.Setenv("CI_PIPELINE_SOURCE", "push")
	t.Setenv("CI_PIPELINE_ID", "1")
	t.Setenv("CI_PIPELINE_IID", "2")

	context, err := ParseGitLabContext()
	assert.NoError(t, err)
	assert.NotNil(t, context)
	assert.Equal(t, PipelineSourceType("push"), context.PipelineSource)
	assert.Equal(t, 1, *context.PipelineId)
	assert.Equal(t, 2, *context.PipelineIId)
	assert.Nil(t, context.MergeRequestId)
	assert.Nil(t, context.MergeRequestIId)
}

func TestOpenMergeRequestEvent(t *testing.T) {
	t.Setenv("CI_PIPELINE_SOURCE", "push")
	t.Setenv("CI_PIPELINE_ID", "1")
	t.Setenv("CI_PIPELINE_IID", "2")

	context, err := ParseGitLabContext()
	assert.NoError(t, err)
	assert.NotNil(t, context)
	assert.Equal(t, PipelineSourceType("push"), context.PipelineSource)
	assert.Equal(t, 1, *context.PipelineId)
	assert.Equal(t, 2, *context.PipelineIId)
	assert.Nil(t, context.MergeRequestId)
	assert.Nil(t, context.MergeRequestIId)
}
