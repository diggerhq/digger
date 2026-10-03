package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestJWTBearerTokenAuthSetsJobTokenForJobTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Organisation{}, &models.JobToken{}))
	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = previousDB })

	jobToken, err := models.DB.CreateDiggerJobToken(1)
	require.NoError(t, err)

	r := gin.New()
	r.GET("/", JWTBearerTokenAuth(services.Auth{}), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(JOB_TOKEN_KEY))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+jobToken.Value)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, jobToken.Value, w.Body.String())
}
