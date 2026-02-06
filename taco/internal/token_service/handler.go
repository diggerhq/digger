package token_service

import (
	"net/http"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/logging"
	querytypes "github.com/diggerhq/digger/opentaco/cmd/token_service/query/types"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for token operations
type Handler struct {
	repo *TokenRepository
}

// NewHandler creates a new token handler
func NewHandler(repo *TokenRepository) *Handler {
	return &Handler{repo: repo}
}

// CreateTokenRequest represents the request to create a new token
type CreateTokenRequest struct {
	UserID    string  `json:"user_id" validate:"required"`
	OrgID     string  `json:"org_id" validate:"required"`
	Name      string  `json:"name"`
	ExpiresIn *string `json:"expires_in"` // Duration string like "24h", "7d", etc.
}

// TokenResponse represents the response for a token
type TokenResponse struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	OrgID      string     `json:"org_id"`
	Token      string     `json:"token"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// VerifyTokenRequest represents the request to verify a token
type VerifyTokenRequest struct {
	Token  string `json:"token" validate:"required"`
	UserID string `json:"user_id"`
	OrgID  string `json:"org_id"`
}

// DeleteTokenRequest represents the request to delete a token
type DeleteTokenRequest struct {
	OrgID string `json:"org_id" validate:"required"`
}

// CreateToken creates a new token
func (h *Handler) CreateToken(c echo.Context) error {
	var req CreateTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.UserID == "" || req.OrgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "user_id and org_id are required"})
	}

	// Parse expiration duration if provided
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn != "" {
		duration, err := time.ParseDuration(*req.ExpiresIn)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid expires_in format. Use duration like '24h' (hours), '30m' (minutes), '168h' (7 days)."})
		}
		exp := time.Now().UTC().Add(duration) // Always use UTC
		expiresAt = &exp
	}

	token, err := h.repo.CreateToken(c.Request().Context(), req.UserID, req.OrgID, req.Name, expiresAt)
	if err != nil {
		logger := logging.FromContext(c)
		logger.Error("Failed to create token", "user_id", req.UserID, "org_id", req.OrgID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create token"})
	}

	// Prevent caching of token creation responses
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Response().Header().Set("Pragma", "no-cache")
	c.Response().Header().Set("Expires", "0")

	return c.JSON(http.StatusCreated, toTokenResponse(token))
}

// ListTokens lists all tokens for an org (org_id is required)
func (h *Handler) ListTokens(c echo.Context) error {
	orgID := c.QueryParam("org_id")
	if orgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id is required"})
	}

	userID := c.QueryParam("user_id") // Optional filter within org

	tokens, err := h.repo.ListTokens(c.Request().Context(), userID, orgID)
	if err != nil {
		logger := logging.FromContext(c)
		logger.Error("Failed to list tokens", "user_id", userID, "org_id", orgID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list tokens"})
	}

	responses := make([]TokenResponse, len(tokens))
	for i, token := range tokens {
		responses[i] = toTokenResponseHidden(token) // Hide token hash
	}

	// Prevent caching of token list responses
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Response().Header().Set("Pragma", "no-cache")
	c.Response().Header().Set("Expires", "0")

	return c.JSON(http.StatusOK, responses)
}

// DeleteToken deletes a token by ID with org ownership validation
func (h *Handler) DeleteToken(c echo.Context) error {
	tokenID := c.Param("id")
	if tokenID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token ID is required"})
	}

	// Parse org_id from request body for ownership validation
	var req DeleteTokenRequest
	if err := c.Bind(&req); err != nil || req.OrgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id is required"})
	}

	// Delete with org validation - returns "not found" if token doesn't belong to org
	if err := h.repo.DeleteTokenForOrg(c.Request().Context(), tokenID, req.OrgID); err != nil {
		if err.Error() == "token not found" {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Token not found"})
		}
		logger := logging.FromContext(c)
		logger.Error("Failed to delete token", "token_id", tokenID, "org_id", req.OrgID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Prevent caching of delete responses
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Response().Header().Set("Pragma", "no-cache")
	c.Response().Header().Set("Expires", "0")

	return c.JSON(http.StatusOK, map[string]string{"message": "Token deleted successfully"})
}

// VerifyToken verifies a token
func (h *Handler) VerifyToken(c echo.Context) error {
	var req VerifyTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.Token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token is required"})
	}

	token, err := h.repo.VerifyToken(c.Request().Context(), req.Token, req.UserID, req.OrgID)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
		"token": toTokenResponseHidden(token), // Hide token hash in verification response
	})
}

// GetToken retrieves a token by ID with org ownership validation
func (h *Handler) GetToken(c echo.Context) error {
	tokenID := c.Param("id")
	orgID := c.QueryParam("org_id")

	if tokenID == "" || orgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token ID and org_id are required"})
	}

	// Get with org validation - returns "not found" if token doesn't belong to org
	token, err := h.repo.GetTokenForOrg(c.Request().Context(), tokenID, orgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Token not found"})
	}

	return c.JSON(http.StatusOK, toTokenResponseHidden(token)) // Hide token hash
}

// HealthCheck returns the health status of the service
func (h *Handler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
}

// toTokenResponse converts a token model to a response
// Note: Token field will be empty for list/get operations (only shown on creation)
func toTokenResponse(token *querytypes.Token) TokenResponse {
	return TokenResponse{
		ID:         token.ID,
		UserID:     token.UserID,
		OrgID:      token.OrgID,
		Token:      token.Token, // Will be plaintext only on creation, hash on list/get
		Name:       token.Name,
		Status:     token.Status,
		CreatedAt:  token.CreatedAt,
		UpdatedAt:  token.UpdatedAt,
		LastUsedAt: token.LastUsedAt,
		ExpiresAt:  token.ExpiresAt,
	}
}

// toTokenResponseHidden converts a token model to a response without showing the token
// Shows last 5 chars of hash for identification (e.g., "abc12")
func toTokenResponseHidden(token *querytypes.Token) TokenResponse {
	resp := toTokenResponse(token)
	
	// Show last 5 chars of hash for identification
	if len(token.Token) > 5 {
		resp.Token = token.Token[len(token.Token)-5:]
	} else {
		resp.Token = "" // Empty if hash is too short (shouldn't happen)
	}
	
	return resp
}

