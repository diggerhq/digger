package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"log"
	"net/http"
	"os"
	"strings"
)

type TenantCreatedEvent struct {
	TenantId string `json:"tenantId,omitempty"`
	Name     string `json:"name,omitempty"`
}

func CreateFronteggOrgFromWebhook(c *gin.Context) {
	var json TenantCreatedEvent

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	source := c.GetHeader("x-tenant-source")

	_, err := models.DB.CreateOrganisation(json.Name, source, json.TenantId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organisation"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func AssociateTenantIdToDiggerOrg(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		c.String(http.StatusForbidden, "No Authorization header provided")
		c.Abort()
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		c.String(http.StatusForbidden, "Could not find bearer token in Authorization header")
		c.Abort()
		return
	}

	jwtPublicKey := os.Getenv("JWT_PUBLIC_KEY")
	if jwtPublicKey == "" {
		log.Printf("No JWT_PUBLIC_KEY environment variable provided")
		c.String(http.StatusInternalServerError, "Error occurred while reading public key")
		c.Abort()
		return
	}
	publicKeyData := []byte(jwtPublicKey)

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
	if err != nil {
		log.Printf("Error while parsing public key: %v", err.Error())
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
		log.Printf("can't parse a token, %v\n", err)
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if token.Valid {

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if claims.Valid() != nil {
				log.Printf("Token's claim is invalid")
				log.Printf("token is invalid")
				c.AbortWithStatus(http.StatusForbidden)
			}

			name := claims["name"]
			tenantId := claims["tenantId"]

			if name == nil {
				log.Printf("claim's name is nil")
				log.Printf("token is invalid (name absent from claim)")
				c.AbortWithStatus(http.StatusForbidden)
			}

			if tenantId == nil {
				log.Printf("claim's tenantId is nil")
				log.Printf("token is invalid (tenantId absent from claim)")
				c.AbortWithStatus(http.StatusForbidden)
			}

			tenantIdStr := tenantId.(string)
			nameStr := name.(string)
			log.Printf("name: %s", name)
			log.Printf("tenantId: %s", tenantId)

			org, err := models.DB.GetOrganisation(tenantId)

			if err != nil {
				log.Printf("Failed to get organisation by tenantId: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
			}

			if org == nil {
				models.DB.CreateOrganisation(nameStr, "", tenantIdStr)
			}

			c.AbortWithStatus(http.StatusOK)
		} else if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				log.Println("That's not even a token")
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				log.Println("Token is either expired or not active yet")
			} else {
				log.Println("Couldn't handle this token:", err)
			}
			c.AbortWithStatus(http.StatusForbidden)
		} else {
			log.Println("Couldn't handle this token:", err)
			c.AbortWithStatus(http.StatusForbidden)
		}
	}
	c.AbortWithStatus(http.StatusForbidden)
}
