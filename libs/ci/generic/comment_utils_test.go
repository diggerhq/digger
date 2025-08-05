package generic

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseDiggerCommentFlags(t *testing.T) {

	comment := "digger plan -p test2"
	parts, valid, err := ParseDiggerCommentFlags(comment)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, parts.Projects, []string{"test2"})
	assert.Equal(t, parts.Directories, []string(nil))
	assert.Equal(t, parts.Layer, -1)

	comment = "digger plan -p test2 -p yesplease"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, parts.Projects, []string{"test2", "yesplease"})
	assert.Equal(t, parts.Directories, []string(nil))
	assert.Equal(t, parts.Layer, -1)

	comment = "digger plan -d dev/vpc -d dev/ec2"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, parts.Projects, []string(nil))
	assert.Equal(t, parts.Directories, []string{"dev/vpc", "dev/ec2"})
	assert.Equal(t, parts.Layer, -1)

	comment = "digger plan -p myproject -d dev/vpc -d dev/ec2"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, parts.Projects, []string{"myproject"})
	assert.Equal(t, parts.Directories, []string{"dev/vpc", "dev/ec2"})
	assert.Equal(t, parts.Layer, -1)

	comment = "digger plan -project myproject --directory dev/vpc --directory dev/ec2"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, parts.Projects, []string{"myproject"})
	assert.Equal(t, parts.Directories, []string{"dev/vpc", "dev/ec2"})
	assert.Equal(t, parts.Layer, -1)

	comment = "digger plan --layer 2"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.NoError(t, err)
	assert.Equal(t, parts.Projects, []string(nil))
	assert.Equal(t, parts.Directories, []string(nil))
	assert.Equal(t, parts.Layer, 2)

	// cant specify layer more than once
	comment = "digger plan --layer 1 --layer 2"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.Error(t, err)

	// cant mix layer and project
	// cant mix layer and project
	comment = "digger plan --layer 1 --project whodat"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.Error(t, err)

	// cant mix layer and directory
	// cant specify layer more than once
	comment = "digger plan --layer 1 --directory whodat"
	parts, valid, err = ParseDiggerCommentFlags(comment)
	assert.Error(t, err)

}
