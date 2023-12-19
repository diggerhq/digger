package middleware

import (
	"github.com/gin-gonic/gin"
)

func NoopWebAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ORGANISATION_ID_KEY, 1)
		c.Next()
	}
}

func NoopApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
