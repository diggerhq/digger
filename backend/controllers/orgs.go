package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
)

type TenantCreatedEvent struct {
	TenantId string `json:"tenantId,omitempty"`
	Name     string `json:"name,omitempty"`
}

func GetOrgSettingsApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId, "source", organisationSource)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Error fetching organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.String(http.StatusInternalServerError, "Error fetching organisation")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"drift_enabled":     org.DriftEnabled,
		"drift_cron_tab":    org.DriftCronTab,
		"drift_webhook_url": org.DriftWebhookUrl,
		"billing_plan":      org.BillingPlan,
	})
}

func UpdateOrgSettingsApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId, "source", organisationSource)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Error fetching organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.String(http.StatusInternalServerError, "Error fetching organisation")
		}
		return
	}
	var reqBody struct {
		DriftEnabled                *bool   `json:"drift_enabled,omitempty"`
		DriftCronTab                *string `json:"drift_cron_tab,omitempty"`
		DriftWebhookUrl             *string `json:"drift_webhook_url,omitempty"`
		BillingPlan                 *string `json:"billing_plan,omitempty"`
		BillingStripeSubscriptionId *string `json:"billing_stripe_subscription_id,omitempty"`
	}
	err = json.NewDecoder(c.Request.Body).Decode(&reqBody)
	if err != nil {
		slog.Error("Error decoding request body", "error", err)
		c.String(http.StatusBadRequest, "Error decoding request body")
		return
	}

	if reqBody.DriftEnabled != nil {
		org.DriftEnabled = *reqBody.DriftEnabled
	}

	if reqBody.DriftCronTab != nil {
		org.DriftCronTab = *reqBody.DriftCronTab
	}

	if reqBody.DriftWebhookUrl != nil {
		org.DriftWebhookUrl = *reqBody.DriftWebhookUrl
	}

	if reqBody.BillingPlan != nil {
		org.BillingPlan = models.BillingPlan(*reqBody.BillingPlan)
	}

	if reqBody.BillingStripeSubscriptionId != nil {
		org.BillingStripeSubscriptionId = *reqBody.BillingStripeSubscriptionId
	}

	err = models.DB.GormDB.Save(&org).Error
	if err != nil {
		slog.Error("Error saving organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
		c.String(http.StatusInternalServerError, "Error saving organisation")
		return
	}

	c.JSON(http.StatusOK, gin.H{})
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
