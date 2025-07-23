package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
)

func NoopWebAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		setDefaultOrganisationId(c)
		c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
		c.Next()
	}
}

func NoopApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")

		token := strings.TrimPrefix(authHeader, "Bearer ")

		if strings.HasPrefix(token, "cli:") {
			slog.Debug("Processing CLI token")
			if jobToken, err := CheckJobToken(c, token); err != nil {
				slog.Warn("Invalid job token", "error", err)
				c.String(http.StatusForbidden, err.Error())
				c.Abort()
				return
			} else {
				c.Set(ORGANISATION_ID_KEY, jobToken.OrganisationID)
				c.Set(ACCESS_LEVEL_KEY, jobToken.Type)
				c.Set(JOB_TOKEN_KEY, jobToken.Value)
				slog.Debug("Job token verified", "organisationId", jobToken.OrganisationID, "accessLevel", jobToken.Type)
				c.Next()
			}
		}
		setDefaultOrganisationId(c)
		c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
		c.Next()
	}
}
