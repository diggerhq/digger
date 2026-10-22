package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecretMatches(t *testing.T) {
	assert.True(t, secretMatches("s3cret", "s3cret"))
	assert.False(t, secretMatches("s3cret", "s3cre"))
	assert.False(t, secretMatches("s3cret", ""))
	assert.False(t, secretMatches("", ""), "an unset secret must not match an empty token")
}

func requestWithAuthorization(t *testing.T, handler gin.HandlerFunc, authorization string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/", handler, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", authorization)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestInternalApiAuth(t *testing.T) {
	t.Setenv("DIGGER_INTERNAL_SECRET", "s3cret")
	assert.Equal(t, http.StatusOK, requestWithAuthorization(t, InternalApiAuth(), "Bearer s3cret"))
	assert.Equal(t, http.StatusForbidden, requestWithAuthorization(t, InternalApiAuth(), "Bearer wrong"))
}

func TestHttpBasicApiAuthRejectsWrongAdminToken(t *testing.T) {
	t.Setenv("BEARER_AUTH_TOKEN", "admin-token")
	assert.Equal(t, http.StatusForbidden, requestWithAuthorization(t, HttpBasicApiAuth(), "Bearer not-the-token"))
}

// net/http trims header whitespace, so a real request can't send an empty
// bearer token. httptest doesn't, which lets these tests reach that case.
func TestInternalApiAuthRejectsEmptyTokenWhenSecretIsUnset(t *testing.T) {
	t.Setenv("DIGGER_INTERNAL_SECRET", "")
	assert.Equal(t, http.StatusForbidden, requestWithAuthorization(t, InternalApiAuth(), "Bearer "))
}

func TestHttpBasicApiAuthRejectsEmptyTokenWhenAdminTokenIsUnset(t *testing.T) {
	t.Setenv("BEARER_AUTH_TOKEN", "")
	assert.Equal(t, http.StatusForbidden, requestWithAuthorization(t, HttpBasicApiAuth(), "Bearer "))
}
