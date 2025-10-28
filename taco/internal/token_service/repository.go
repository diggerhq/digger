package token_service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

const (
	errTokenNotFound = "token not found"
	queryTokenByID   = "id = ?"
)

// TokenRepository handles token CRUD operations
type TokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(db *gorm.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

// CreateToken creates a new token for a given user ID and org
func (r *TokenRepository) CreateToken(ctx context.Context, userID, orgID, name string, expiresAt *time.Time) (*types.Token, error) {
	// Generate secure random token
	tokenValue, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	token := &types.Token{
		UserID:    userID,
		OrgID:     orgID,
		Token:     tokenValue,
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	return token, nil
}

// ListTokens returns all tokens for a given user ID and org
func (r *TokenRepository) ListTokens(ctx context.Context, userID, orgID string) ([]*types.Token, error) {
	var tokens []*types.Token
	query := r.db.WithContext(ctx)

	// Filter by userID if provided
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// Filter by orgID if provided
	if orgID != "" {
		query = query.Where("org_id = ?", orgID)
	}

	if err := query.Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}

	return tokens, nil
}

// DeleteToken deletes a token by ID
func (r *TokenRepository) DeleteToken(ctx context.Context, tokenID string) error {
	result := r.db.WithContext(ctx).Delete(&types.Token{}, queryTokenByID, tokenID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New(errTokenNotFound)
	}
	return nil
}

// VerifyToken verifies a token by token value, userID, and orgID
func (r *TokenRepository) VerifyToken(ctx context.Context, tokenValue, userID, orgID string) (*types.Token, error) {
	var token types.Token
	query := r.db.WithContext(ctx).Where("token = ?", tokenValue)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if orgID != "" {
		query = query.Where("org_id = ?", orgID)
	}

	if err := query.First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errTokenNotFound)
		}
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}

	// Check if token is active
	if token.Status != "active" {
		return nil, errors.New("token is not active")
	}

	// Check if token has expired
	if token.ExpiresAt != nil && time.Now().After(*token.ExpiresAt) {
		return nil, errors.New("token has expired")
	}

	// Update last used time asynchronously
	go func() {
		now := time.Now()
		_ = r.db.Model(&types.Token{}).Where(queryTokenByID, token.ID).Update("last_used_at", now).Error
	}()

	return &token, nil
}

// GetToken retrieves a token by ID
func (r *TokenRepository) GetToken(ctx context.Context, tokenID string) (*types.Token, error) {
	var token types.Token
	if err := r.db.WithContext(ctx).Where(queryTokenByID, tokenID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errTokenNotFound)
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	return &token, nil
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "otc_tok_" + base64.URLEncoding.EncodeToString(b), nil
}

