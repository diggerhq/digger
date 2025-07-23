package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HeadersApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgId := c.Request.Header.Get("DIGGER_ORG_ID")
		orgSource := c.Request.Header.Get("DIGGER_ORG_SOURCE")
		userId := c.Request.Header.Get("DIGGER_USER_ID")

		if orgId == "" {
			c.String(http.StatusBadRequest, "Missing parameter: DIGGER_ORG_ID")
			c.Abort()
			return
		}

		if orgSource == "" {
			c.String(http.StatusBadRequest, "Missing parameter: DIGGER_ORG_SOURCE")
			c.Abort()
			return
		}

		if userId == "" {
			c.String(http.StatusBadRequest, "Missing parameter: DIGGER_USER_ID")
			c.Abort()
			return
		}

		c.Set(ORGANISATION_ID_KEY, orgId)
		c.Set(ORGANISATION_SOURCE_KEY, orgSource)
		c.Set(USER_ID_KEY, userId)

		c.Next()
	}
}
