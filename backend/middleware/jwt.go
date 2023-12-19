package middleware

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"log"
	"net/http"
	"os"
	"strings"
)

func SetContextParameters(c *gin.Context, auth services.Auth, token *jwt.Token) error {
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if claims.Valid() != nil {
			log.Printf("Token's claim is invalid")
			return fmt.Errorf("token is invalid")
		}
		var org *models.Organisation
		tenantId := claims["tenantId"]
		if tenantId == nil {
			log.Printf("claim's tenantId is nil")
			return fmt.Errorf("token is invalid")
		}
		tenantId = tenantId.(string)
		log.Printf("tenantId: %s", tenantId)

		org, err := models.DB.GetOrganisation(tenantId)
		if err != nil {
			log.Printf("Error while fetching organisation: %v", err)
			return err
		} else if org == nil {
			log.Printf("No organisation found for tenantId %s", tenantId)
			return fmt.Errorf("token is invalid")
		}

		c.Set(ORGANISATION_ID_KEY, org.ID)

		log.Printf("set org id %v\n", org.ID)

		tokenType := claims["type"].(string)

		permissions := make([]string, 0)
		if tokenType == "tenantAccessToken" {
			permission, err := auth.FetchTokenPermissions(claims["sub"].(string))
			if err != nil {
				log.Printf("Error while fetching permissions: %v", err)
				return fmt.Errorf("token is invalid")
			}
			permissions = permission
		} else {
			permissionsClaims := claims["permissions"]
			if permissionsClaims == nil {
				log.Printf("claim's permissions is nil")
				return fmt.Errorf("token is invalid")
			}
			for _, permissionClaim := range permissionsClaims.([]interface{}) {
				permissions = append(permissions, permissionClaim.(string))
			}
		}
		for _, permission := range permissions {
			if permission == "digger.all.*" {
				c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
				return nil
			}
		}
		for _, permission := range permissions {
			if permission == "digger.all.read.*" {
				c.Set(ACCESS_LEVEL_KEY, models.AccessPolicyType)
				return nil
			}
		}
	} else {
		log.Printf("Token's claim is invalid")
		return fmt.Errorf("token is invalid")
	}
	return nil
}

func WebAuth(auth services.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string
		tokenString, err := c.Cookie("token")
		if err != nil {
			log.Printf("can't get a cookie token, %v\n", err)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		if tokenString == "" {
			log.Println("auth token is empty")
			c.AbortWithStatus(http.StatusForbidden)
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
			err = SetContextParameters(c, auth, token)
			if err != nil {
				log.Printf("Error while setting context parameters: %v", err)
				c.String(http.StatusForbidden, "Failed to parse token")
				c.Abort()
				return
			}

			c.Next()
			return
		} else if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				log.Println("That's not even a token")
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				log.Println("Token is either expired or not active yet")
			} else {
				log.Println("Couldn't handle this token:", err)
			}
		} else {
			log.Println("Couldn't handle this token:", err)
		}

		c.AbortWithStatus(http.StatusForbidden)
	}
}

func SecretCodeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.Request.Header.Get("x-webhook-secret")
		if secret == "" {
			log.Printf("No x-webhook-secret header provided")
			c.String(http.StatusForbidden, "No x-webhook-secret header provided")
			c.Abort()
			return
		}
		_, err := jwt.Parse(secret, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("WEBHOOK_SECRET")), nil
		})

		if err != nil {
			log.Printf("Error parsing secret: %v", err.Error())
			c.String(http.StatusForbidden, "Invalid x-webhook-secret header provided")
			c.Abort()
			return
		}
		c.Next()
	}
}

func BearerTokenAuth(auth services.Auth) gin.HandlerFunc {
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

		if strings.HasPrefix(token, "t:") {
			var dbToken models.Token

			token, err := models.DB.GetToken(token)
			if token == nil {
				c.String(http.StatusForbidden, "Invalid bearer token")
				c.Abort()
				return
			}

			if err != nil {
				log.Printf("Error while fetching token from database: %v", err)
				c.String(http.StatusInternalServerError, "Error occurred while fetching database")
				c.Abort()
				return
			}
			c.Set(ORGANISATION_ID_KEY, dbToken.OrganisationID)
			c.Set(ACCESS_LEVEL_KEY, dbToken.Type)
		} else {
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

			token, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return publicKey, nil
			})

			if err != nil {
				log.Printf("Error while parsing token: %v", err.Error())
				c.String(http.StatusForbidden, "Authorization header is invalid")
				c.Abort()
				return
			}

			if !token.Valid {
				log.Printf("Token is invalid")
				c.String(http.StatusForbidden, "Authorization header is invalid")
				c.Abort()
				return
			}

			err = SetContextParameters(c, auth, token)
			if err != nil {
				log.Printf("Error while setting context parameters: %v", err)
				c.String(http.StatusForbidden, "Failed to parse token")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func AccessLevel(allowedAccessLevels ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessLevel := c.GetString(ACCESS_LEVEL_KEY)
		for _, allowedAccessLevel := range allowedAccessLevels {
			if accessLevel == allowedAccessLevel {
				c.Next()
				return
			}
		}
		c.String(http.StatusForbidden, "Not allowed to access this resource with this access level")
		c.Abort()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

const ORGANISATION_ID_KEY = "organisation_ID"
const ACCESS_LEVEL_KEY = "access_level"
