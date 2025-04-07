package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
)

func HttpBasicWebAuth() gin.HandlerFunc {

	return func(c *gin.Context) {
		slog.Info("Restricting access with HTTP Basic Auth")
		username := os.Getenv("HTTP_BASIC_AUTH_USERNAME")
		password := os.Getenv("HTTP_BASIC_AUTH_PASSWORD")
		if username == "" || password == "" {
			err := fmt.Errorf("configuration error: HTTP Basic Auth configured but username or password not set")
			slog.Error("Basic auth configuration error", "error", err)
			c.Error(err)
		}
		gin.BasicAuth(gin.Accounts{
			username: password,
		})(c)
		c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
		setDefaultOrganisationId(c)
		c.Next()
	}
}

func setDefaultOrganisationId(c *gin.Context) {
	orgNumberOne, err := models.DB.GetOrganisation(models.DEFAULT_ORG_NAME)
	if err != nil {
		errMsg := fmt.Errorf("Error fetching default organisation please check your configuration")
		slog.Error("Could not fetch default organisation", "defaultOrgName", models.DEFAULT_ORG_NAME, "error", err)
		c.Error(errMsg)
		return
	}
	slog.Debug("Setting default organisation ID", "orgId", orgNumberOne.ID)
	c.Set(ORGANISATION_ID_KEY, orgNumberOne.ID)
}

func HttpBasicApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			slog.Warn("No Authorization header provided")
			c.String(http.StatusForbidden, "No Authorization header provided")
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			slog.Warn("Could not find bearer token in Authorization header")
			c.String(http.StatusForbidden, "Could not find bearer token in Authorization header")
			c.Abort()
			return
		}

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
			}
		} else if token == os.Getenv("BEARER_AUTH_TOKEN") {
			slog.Debug("Using admin bearer token")
			setDefaultOrganisationId(c)
			c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
			c.Next()
		} else {
			slog.Warn("Invalid Bearer token")
			c.String(http.StatusForbidden, "Invalid Bearer token")
			c.Abort()
			return
		}
		return
	}
}
