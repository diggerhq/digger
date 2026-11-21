package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// useTestDatabase points models.DB at an SQLite database holding the default
// organisation, which the admin bearer path looks up, and returns that
// organisation.
func useTestDatabase(t *testing.T) *models.Organisation {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Organisation{}, &models.JobToken{}))

	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = previousDB })

	// Same shape as the default organisation models.ConnectDatabase creates.
	org, err := models.DB.CreateOrganisation("digger", "", models.DEFAULT_ORG_NAME, nil)
	require.NoError(t, err)
	return org
}

func requestGetSpec(t *testing.T, authorization string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_AUTH", "")
	t.Setenv("HTTP_BASIC_AUTH", "true")
	t.Setenv("BEARER_AUTH_TOKEN", "admin-token")

	r := gin.New()
	registerGetSpec(r, func(c *gin.Context) {
		c.String(http.StatusOK, "spec")
	})

	req := httptest.NewRequest(http.MethodPost, "/get-spec", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestGetSpecRejectsMissingAuthorization(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, requestGetSpec(t, ""))
}

func TestGetSpecRejectsWrongBearerToken(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, requestGetSpec(t, "Bearer not-the-token"))
}

func TestGetSpecAllowsAdminBearerToken(t *testing.T) {
	useTestDatabase(t)
	assert.Equal(t, http.StatusOK, requestGetSpec(t, "Bearer admin-token"))
}

func TestGetSpecRejectsJobToken(t *testing.T) {
	org := useTestDatabase(t)
	jobToken, err := models.DB.CreateDiggerJobToken(org.ID)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, requestGetSpec(t, "Bearer "+jobToken.Value))
}
