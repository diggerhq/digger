package middleware

import (
	"fmt"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"time"
)

func CheckJobToken(c *gin.Context, token string) (*model.DiggerJobToken, error) {
	jobToken, err := dbmodels.DB.GetJobToken(token)
	if jobToken == nil {
		c.String(http.StatusForbidden, "Invalid bearer token")
		c.Abort()
		return nil, fmt.Errorf("invalid bearer token")
	}

	if time.Now().After(jobToken.Expiry) {
		log.Printf("Token has already expired: %v", err)
		c.String(http.StatusForbidden, "Token has expired")
		c.Abort()
		return nil, fmt.Errorf("token has expired")
	}

	if err != nil {
		log.Printf("Error while fetching token from database: %v", err)
		c.String(http.StatusInternalServerError, "Error occurred while fetching database")
		c.Abort()
		return nil, fmt.Errorf("could not fetch cli token")
	}

	return jobToken, nil
}

func JobTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.String(http.StatusForbidden, "No Authorization header provided")
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.String(http.StatusForbidden, "Could not find bearer token in Authorization header")
			c.Abort()
			return
		}

		if strings.HasPrefix(token, "cli:") {
			if jobToken, err := CheckJobToken(c, token); err != nil {
				c.String(http.StatusForbidden, err.Error())
				c.Abort()
				return
			} else {
				c.Set(jobToken.OrganisationID, jobToken.OrganisationID)
				c.Set(ACCESS_LEVEL_KEY, jobToken.Type)
			}
		} else {
			c.String(http.StatusForbidden, "Invalid Bearer token")
			c.Abort()
			return
		}
		return
	}
}
