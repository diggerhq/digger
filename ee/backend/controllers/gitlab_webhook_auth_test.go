package controllers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sendGitlabWebhook sends a push event, which the handler accepts without
// calling GitLab. Rejected requests never reach the database.
func sendGitlabWebhook(t *testing.T, token string, setToken bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/gitlab-webhook", strings.NewReader(`{"object_kind":"push"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	if setToken {
		req.Header.Set("X-Gitlab-Token", token)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	DiggerEEController{}.GitlabWebHookHandler(c)
	return w
}

func TestGitlabWebHookRejectsWhenSecretIsUnset(t *testing.T) {
	t.Setenv("DIGGER_GITLAB_WEBHOOK_SECRET", "")
	w := sendGitlabWebhook(t, "", false)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitlabWebHookRejectsWhenSecretIsUnsetAndTokenIsEmpty(t *testing.T) {
	t.Setenv("DIGGER_GITLAB_WEBHOOK_SECRET", "")
	w := sendGitlabWebhook(t, "", true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitlabWebHookRejectsMissingToken(t *testing.T) {
	t.Setenv("DIGGER_GITLAB_WEBHOOK_SECRET", "s3cret")
	w := sendGitlabWebhook(t, "", false)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitlabWebHookRejectsWrongToken(t *testing.T) {
	t.Setenv("DIGGER_GITLAB_WEBHOOK_SECRET", "s3cret")
	w := sendGitlabWebhook(t, "not-the-secret", true)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitlabWebHookAcceptsCorrectToken(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Organisation{}))
	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = previousDB })
	_, err = models.DB.CreateOrganisation("digger", "", models.DEFAULT_ORG_NAME, nil)
	require.NoError(t, err)

	t.Setenv("DIGGER_GITLAB_WEBHOOK_SECRET", "s3cret")
	w := sendGitlabWebhook(t, "s3cret", true)
	assert.Equal(t, http.StatusOK, w.Code)
}
