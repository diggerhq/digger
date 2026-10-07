package controllers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testBitbucketWebhookSecret = "s3cret"

// setupBitbucketConnection stores a Bitbucket VCS connection whose webhook
// secret is encrypted the way the handler expects, and returns its ID.
func setupBitbucketConnection(t *testing.T) string {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Organisation{}, &models.VCSConnection{}))

	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = previousDB })

	encryptionKey := "0123456789abcdef0123456789abcdef"
	t.Setenv("DIGGER_ENCRYPTION_SECRET", encryptionKey)

	encrypt := func(plaintext string) string {
		ciphertext, err := utils.AESEncrypt([]byte(encryptionKey), plaintext)
		require.NoError(t, err)
		return ciphertext
	}

	connection, err := models.DB.CreateVCSConnection("bitbucket", models.DiggerVCSBitbucket, 0, "", "", "", "", "", "", "",
		encrypt("access-token"), encrypt(testBitbucketWebhookSecret), "", "", 1)
	require.NoError(t, err)
	return fmt.Sprint(connection.ID)
}

func sendBitbucketWebhook(t *testing.T, connectionID string, body []byte, signature string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/bitbucket-webhook", bytes.NewReader(body))
	req.Header.Set("X-Event-Key", "pullrequest:created")
	req.Header.Set("DIGGER_CONNECTION_ID", connectionID)
	if signature != "" {
		req.Header.Set("X-Hub-Signature", signature)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	DiggerEEController{}.BitbucketWebhookHandler(c)
	return w
}

func signBitbucketPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestBitbucketWebhookRejectsUnsignedPayload(t *testing.T) {
	connectionID := setupBitbucketConnection(t)
	w := sendBitbucketWebhook(t, connectionID, []byte(`{}`), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBitbucketWebhookRejectsWrongSignature(t *testing.T) {
	connectionID := setupBitbucketConnection(t)
	body := []byte(`{}`)
	w := sendBitbucketWebhook(t, connectionID, body, signBitbucketPayload("not-the-secret", body))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBitbucketWebhookAcceptsSignedPayload(t *testing.T) {
	connectionID := setupBitbucketConnection(t)
	body := []byte(`{}`)
	w := sendBitbucketWebhook(t, connectionID, body, signBitbucketPayload(testBitbucketWebhookSecret, body))
	assert.Equal(t, http.StatusAccepted, w.Code)
}
