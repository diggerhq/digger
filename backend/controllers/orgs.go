package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type TenantCreatedEvent struct {
	TenantId string `json:"tenantId,omitempty"`
	Name     string `json:"name,omitempty"`
}

func CreateFronteggOrgFromWebhook(c *gin.Context) {
	var json TenantCreatedEvent

	if err := c.ShouldBindJSON(&json); err != nil {
		slog.Error("Failed to bind JSON", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	source := c.GetHeader("x-tenant-source")

	org, err := models.DB.CreateOrganisation(json.Name, source, json.TenantId)
	if err != nil {
		slog.Error("Failed to create organisation", "tenantId", json.TenantId, "name", json.Name, "source", source, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organisation"})
		return
	}

	slog.Info("Successfully created organisation from webhook", "tenantId", json.TenantId, "name", json.Name, "orgId", org.ID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func AssociateTenantIdToDiggerOrg(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		slog.Warn("No Authorization header provided")
		c.String(http.StatusForbidden, "No Authorization header provided")
		c.Abort()
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		slog.Warn("Could not find bearer token in Authorization header")
		c.String(http.StatusForbidden, "Could not find bearer token in Authorization header")
		c.Abort()
		return
	}

	jwtPublicKey := os.Getenv("JWT_PUBLIC_KEY")
	if jwtPublicKey == "" {
		slog.Error("No JWT_PUBLIC_KEY environment variable provided")
		c.String(http.StatusInternalServerError, "Error occurred while reading public key")
		c.Abort()
		return
	}
	publicKeyData := []byte(jwtPublicKey)

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
	if err != nil {
		slog.Error("Error while parsing public key", "error", err)
		c.String(http.StatusInternalServerError, "Error occurred while parsing public key")
		c.Abort()
		return
	}

	// validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		slog.Error("Can't parse token", "error", err)
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if token.Valid {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if claims.Valid() != nil {
				slog.Warn("Token's claim is invalid")
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

			name := claims["name"]
			tenantId := claims["tenantId"]

			if name == nil {
				slog.Warn("Token is invalid - name absent from claim")
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

			if tenantId == nil {
				slog.Warn("Token is invalid - tenantId absent from claim")
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

			tenantIdStr := tenantId.(string)
			nameStr := name.(string)
			slog.Debug("Processing JWT claims", "name", nameStr, "tenantId", tenantIdStr)

			org, err := models.DB.GetOrganisation(tenantId)

			if err != nil {
				slog.Error("Failed to get organisation by tenantId", "tenantId", tenantIdStr, "error", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			if org == nil {
				newOrg, err := models.DB.CreateOrganisation(nameStr, "", tenantIdStr)
				if err != nil {
					slog.Error("Failed to create organisation", "tenantId", tenantIdStr, "name", nameStr, "error", err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				slog.Info("Created new organisation", "tenantId", tenantIdStr, "name", nameStr, "orgId", newOrg.ID)
			} else {
				slog.Info("Organisation already exists", "tenantId", tenantIdStr, "orgId", org.ID)
			}

			c.AbortWithStatus(http.StatusOK)
			return
		} else if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				slog.Warn("That's not even a token")
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				slog.Warn("Token is either expired or not active yet")
			} else {
				slog.Error("Couldn't handle token", "error", err)
			}
			c.AbortWithStatus(http.StatusForbidden)
			return
		} else {
			slog.Error("Couldn't handle token", "error", err)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
	}

	slog.Warn("Token is invalid")
	c.AbortWithStatus(http.StatusForbidden)
}
