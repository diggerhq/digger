package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
)

func WebhookAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		webhookSecret := os.Getenv("DIGGER_WEBHOOK_SECRET")
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.String(http.StatusForbidden, "No Authorization header provided")
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != webhookSecret {
			c.String(http.StatusForbidden, "invalid token")
			c.Abort()
			return
		}
		c.Next()
		return
	}
}
