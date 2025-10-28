package token_service

import (
	"log"
	"net/http"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
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
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid expires_in format. Use duration format like '24h', '7d'"})
		}
		exp := time.Now().Add(duration)
		expiresAt = &exp
	}

	token, err := h.repo.CreateToken(c.Request().Context(), req.UserID, req.OrgID, req.Name, expiresAt)
	if err != nil {
		log.Printf("Failed to create token: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create token"})
	}

	return c.JSON(http.StatusCreated, toTokenResponse(token))
}

// ListTokens lists all tokens for a user and org
func (h *Handler) ListTokens(c echo.Context) error {
	userID := c.QueryParam("user_id")
	orgID := c.QueryParam("org_id")

	tokens, err := h.repo.ListTokens(c.Request().Context(), userID, orgID)
	if err != nil {
		log.Printf("Failed to list tokens: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list tokens"})
	}

	responses := make([]TokenResponse, len(tokens))
	for i, token := range tokens {
		responses[i] = toTokenResponse(token)
	}

	return c.JSON(http.StatusOK, responses)
}

// DeleteToken deletes a token by ID
func (h *Handler) DeleteToken(c echo.Context) error {
	tokenID := c.Param("id")
	if tokenID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token ID is required"})
	}

	if err := h.repo.DeleteToken(c.Request().Context(), tokenID); err != nil {
		log.Printf("Failed to delete token: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

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
		"token": toTokenResponse(token),
	})
}

// GetToken retrieves a token by ID
func (h *Handler) GetToken(c echo.Context) error {
	tokenID := c.Param("id")
	if tokenID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token ID is required"})
	}

	token, err := h.repo.GetToken(c.Request().Context(), tokenID)
	if err != nil {
		log.Printf("Failed to get token: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, toTokenResponse(token))
}

// HealthCheck returns the health status of the service
func (h *Handler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
}

// toTokenResponse converts a token model to a response
func toTokenResponse(token *types.Token) TokenResponse {
	return TokenResponse{
		ID:         token.ID,
		UserID:     token.UserID,
		OrgID:      token.OrgID,
		Token:      token.Token,
		Name:       token.Name,
		Status:     token.Status,
		CreatedAt:  token.CreatedAt,
		UpdatedAt:  token.UpdatedAt,
		LastUsedAt: token.LastUsedAt,
		ExpiresAt:  token.ExpiresAt,
	}
}

