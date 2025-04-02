package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/go-substrate/strate/backend/models"
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
		setDefaultOrganisationId(c)
		c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
		c.Next()
	}
}
