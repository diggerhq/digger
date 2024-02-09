package orchestrator

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsPlanForDiggerPlanJobCorrect(t *testing.T) {
	j := JobJson{
		ProjectName:      "project.Name",
		ProjectDir:       "project.Dir",
		ProjectWorkspace: "workspace",
		Terragrunt:       false,
		Commands:         []string{"run echo 'hello", "digger plan"},
		EventName:        "issue_comment",
	}
	assert.True(t, j.IsPlan())
	assert.False(t, j.IsApply())
}

func TestIsApplyForDiggerApplyJobCorrect(t *testing.T) {
	j := JobJson{
		ProjectName:      "project.Name",
		ProjectDir:       "project.Dir",
		ProjectWorkspace: "workspace",
		Terragrunt:       false,
		Commands:         []string{"digger apply"},
		EventName:        "issue_comment",
	}
	assert.True(t, j.IsApply())
	assert.False(t, j.IsPlan())
}
