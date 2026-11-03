package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCommentCommand(t *testing.T) {
	cases := []struct {
		comment string
		command string
		want    bool
	}{
		{"digger apply", "digger apply", true},
		{"  Digger Apply  ", "digger apply", true},
		{"digger apply -p prod", "digger apply", true},
		{"digger apply\n-p prod", "digger apply", true},
		{"digger plan\tprod", "digger plan", true},
		{"digger applyt", "digger apply", false},
		{"digger applying", "digger apply", false},
		{"digger planet", "digger plan", false},
		{"digger unlocked", "digger unlock", false},
		{"lgtm, digger apply", "digger apply", false},
		{"digger", "digger apply", false},
		{"", "digger apply", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, IsCommentCommand(c.comment, c.command), "%q vs %q", c.comment, c.command)
	}
}

func TestGetCommandFromCommentRejectsTypos(t *testing.T) {
	for _, comment := range []string{"digger applyt", "digger planet", "digger unlocked"} {
		_, err := GetCommandFromComment(comment)
		assert.Error(t, err, comment)
	}

	cmd, err := GetCommandFromComment("digger apply -p prod")
	assert.NoError(t, err)
	assert.Equal(t, DiggerCommandApply, *cmd)
}
