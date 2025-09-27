package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func SetContextParameters(c *gin.Context, auth services.Auth, claims jwt.MapClaims) error {
	var org *models.Organisation
	tenantId := claims["tenantId"]
	if tenantId == nil {
		slog.Warn("Claim's tenantId is nil")
		return fmt.Errorf("token is invalid")
	}
	tenantId = tenantId.(string)
	slog.Debug("Processing tenant ID", "tenantId", tenantId)

	org, err := models.DB.GetOrganisation(tenantId)
	if err != nil {
		slog.Error("Error while fetching organisation", "tenantId", tenantId, "error", err)
		return err
	} else if org == nil {
		slog.Warn("No organisation found for tenantId", "tenantId", tenantId)
		return fmt.Errorf("token is invalid")
	}

	c.Set(ORGANISATION_ID_KEY, org.ID)

	segment.GetClient()
	segment.IdentifyClient(strconv.Itoa(int(org.ID)), org.Name, org.Name, org.Name, org.Name, strconv.Itoa(int(org.ID)), "")

	slog.Debug("Set organisation ID in context", "orgId", org.ID)

	tokenType := claims["type"].(string)

	permissions := make([]string, 0)
	if tokenType == "tenantAccessToken" {
		permission, err := auth.FetchTokenPermissions(claims["sub"].(string))
		if err != nil {
			slog.Error("Error while fetching permissions", "subject", claims["sub"].(string), "error", err)
			return fmt.Errorf("token is invalid")
		}
		permissions = permission
	} else {
		permissionsClaims := claims["permissions"]
		if permissionsClaims == nil {
			slog.Warn("Claim's permissions is nil")
			return fmt.Errorf("token is invalid")
		}
		for _, permissionClaim := range permissionsClaims.([]interface{}) {
			permissions = append(permissions, permissionClaim.(string))
		}
	}
	for _, permission := range permissions {
		if permission == "digger.all.*" {
			slog.Debug("Setting admin access level", "permission", permission)
			c.Set(ACCESS_LEVEL_KEY, models.AdminPolicyType)
			return nil
		}
	}
	for _, permission := range permissions {
		if permission == "digger.all.read.*" {
			slog.Debug("Setting read access level", "permission", permission)
			c.Set(ACCESS_LEVEL_KEY, models.AccessPolicyType)
			return nil
		}
	}
	return nil
}

func JWTWebAuth(auth services.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string
		tokenString, err := c.Cookie("token")
		if err != nil {
			slog.Warn("Can't get a cookie token", "error", err)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		if tokenString == "" {
			slog.Warn("Auth token is empty")
			c.AbortWithStatus(http.StatusForbidden)
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
			slog.Warn("Can't parse a token", "error", err)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); token.Valid && ok {
			err = SetContextParameters(c, auth, claims)
			if err != nil {
				slog.Error("Error while setting context parameters", "error", err)
				c.String(http.StatusForbidden, "Failed to parse token")
				c.Abort()
				return
			}

			c.Next()
			return
		} else {
			slog.Warn("Couldn't handle this token", "error", err)
		}

		c.AbortWithStatus(http.StatusForbidden)
	}
}

func SecretCodeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.Request.Header.Get("x-webhook-secret")
		if secret == "" {
			slog.Warn("No x-webhook-secret header provided")
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
			slog.Error("Error parsing webhook secret", "error", err)
			c.String(http.StatusForbidden, "Invalid x-webhook-secret header provided")
			c.Abort()
			return
		}
		slog.Debug("Webhook secret verified successfully")
		c.Next()
	}
}

func JWTBearerTokenAuth(auth services.Auth) gin.HandlerFunc {
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
				slog.Debug("Job token verified", "organisationId", jobToken.OrganisationID, "accessLevel", jobToken.Type)
			}
		} else if strings.HasPrefix(token, "t:") {
			slog.Debug("Processing API token")
			var dbToken models.Token

			tokenObj, err := models.DB.GetToken(token)
			if tokenObj == nil {
				slog.Warn("Invalid bearer token", "token", token)
				c.String(http.StatusForbidden, "Invalid bearer token")
				c.Abort()
				return
			}

			if err != nil {
				slog.Error("Error while fetching token from database", "error", err)
				c.String(http.StatusInternalServerError, "Error occurred while fetching database")
				c.Abort()
				return
			}
			c.Set(ORGANISATION_ID_KEY, dbToken.OrganisationID)
			c.Set(ACCESS_LEVEL_KEY, dbToken.Type)
			slog.Debug("API token verified", "organisationId", dbToken.OrganisationID, "accessLevel", dbToken.Type)
		} else {
			slog.Debug("Processing JWT token")
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

			parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return publicKey, nil
			})

			if err != nil {
				slog.Error("Error while parsing token", "error", err)
				c.String(http.StatusForbidden, "Authorization header is invalid")
				c.Abort()
				return
			}

			claims, ok := parsedToken.Claims.(jwt.MapClaims)
			if !parsedToken.Valid || !ok {
				slog.Warn("Token is invalid")
				c.String(http.StatusForbidden, "Authorization header is invalid")
				c.Abort()
				return
			}

			err = SetContextParameters(c, auth, claims)
			if err != nil {
				slog.Error("Error while setting context parameters", "error", err)
				c.String(http.StatusForbidden, "Failed to parse token")
				c.Abort()
				return
			}
			slog.Debug("JWT token verified successfully")
		}

		c.Next()
	}
}

func AccessLevel(allowedAccessLevels ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessLevel := c.GetString(ACCESS_LEVEL_KEY)
		for _, allowedAccessLevel := range allowedAccessLevels {
			if accessLevel == allowedAccessLevel {
				slog.Debug("Access level authorized", "accessLevel", accessLevel, "allowedLevel", allowedAccessLevel)
				c.Next()
				return
			}
		}
		slog.Warn("Access level not allowed", "accessLevel", accessLevel, "allowedLevels", allowedAccessLevels)
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
const ORGANISATION_SOURCE_KEY = "organisation_Source"
const USER_ID_KEY = "user_ID"
const ACCESS_LEVEL_KEY = "access_level"
const JOB_TOKEN_KEY = "job_token"
