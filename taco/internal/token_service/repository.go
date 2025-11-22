package token_service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/diggerhq/digger/opentaco/cmd/token_service/query/types"
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

	// Hash the token for storage (only hash is stored in DB)
	tokenHash := hashToken(tokenValue)

	now := time.Now().UTC() // Always use UTC for consistency
	token := &types.Token{
		UserID:    userID,
		OrgID:     orgID,
		Token:     tokenHash, // Store hash, not plaintext
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Return the token with plaintext value (only time user sees it)
	// This is safe because the DB stores only the hash
	token.Token = tokenValue
	return token, nil
}

// ListTokens returns tokens for a given user ID and org with pagination.
func (r *TokenRepository) ListTokens(ctx context.Context, userID, orgID string, page, pageSize int) ([]*types.Token, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var tokens []*types.Token
	query := r.db.WithContext(ctx).Model(&types.Token{})

	// Filter by userID if provided
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// Filter by orgID if provided
	if orgID != "" {
		query = query.Where("org_id = ?", orgID)
	}

	// Count total matching tokens (without pagination)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list tokens: %w", err)
	}

	return tokens, total, nil
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
	// Hash the provided token to compare with stored hash
	tokenHash := hashToken(tokenValue)

	var token types.Token
	query := r.db.WithContext(ctx).Where("token = ?", tokenHash)

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
	if token.ExpiresAt != nil && time.Now().UTC().After(*token.ExpiresAt) {
		return nil, errors.New("token has expired")
	}

	// Update last used time asynchronously
	go func() {
		now := time.Now().UTC()
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

// hashToken hashes a token using SHA-256
// This is a one-way hash - tokens cannot be retrieved from the hash
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
