package services

import (
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/stretchr/testify/assert"
)

func TestGitTimeoutFromBatch(t *testing.T) {
	assert.Equal(t, 0, gitTimeoutFromBatch(nil))
	assert.Equal(t, 0, gitTimeoutFromBatch(&models.DiggerBatch{}))
	assert.Equal(t, 0, gitTimeoutFromBatch(&models.DiggerBatch{DiggerConfig: "projects: []\n"}))
	assert.Equal(t, 0, gitTimeoutFromBatch(&models.DiggerBatch{DiggerConfig: "git_timeout: [\n"}))
	assert.Equal(t, 300, gitTimeoutFromBatch(&models.DiggerBatch{DiggerConfig: "git_timeout: 300\nprojects: []\n"}))
}
