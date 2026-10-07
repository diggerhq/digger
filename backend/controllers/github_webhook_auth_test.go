package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diggerhq/digger/backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// webhookCredentialsProvider returns a fixed webhook secret, or an error, from
// FetchCredentials. Everything else comes from the mock provider.
type webhookCredentialsProvider struct {
	utils.DiggerGithubClientMockProvider
	webhookSecret string
	err           error
}

func (p webhookCredentialsProvider) FetchCredentials(string) (string, string, string, string, error) {
	return "clientId", "clientSecret", p.webhookSecret, "", p.err
}

const pingPayload = `{"zen":"Keep it logically awesome.","hook_id":1}`

func sendGithubWebhook(t *testing.T, provider utils.GithubClientProvider, signature string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(pingPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Hook-Installation-Target-ID", "123")
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	DiggerController{GithubClientProvider: provider}.GithubAppWebHook(c)
	return w
}

func signGithubPayload(secret string, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGithubAppWebHookRejectsWhenWebhookSecretIsEmpty(t *testing.T) {
	w := sendGithubWebhook(t, webhookCredentialsProvider{webhookSecret: ""}, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGithubAppWebHookRejectsWhenCredentialLookupFails(t *testing.T) {
	provider := webhookCredentialsProvider{err: errors.New("could not find app")}
	w := sendGithubWebhook(t, provider, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGithubAppWebHookRejectsUnsignedPayload(t *testing.T) {
	w := sendGithubWebhook(t, webhookCredentialsProvider{webhookSecret: "s3cret"}, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGithubAppWebHookRejectsWrongSignature(t *testing.T) {
	provider := webhookCredentialsProvider{webhookSecret: "s3cret"}
	w := sendGithubWebhook(t, provider, signGithubPayload("not-the-secret", pingPayload))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGithubAppWebHookAcceptsSignedPayload(t *testing.T) {
	provider := webhookCredentialsProvider{webhookSecret: "s3cret"}
	w := sendGithubWebhook(t, provider, signGithubPayload("s3cret", pingPayload))
	assert.Equal(t, http.StatusAccepted, w.Code)
}
