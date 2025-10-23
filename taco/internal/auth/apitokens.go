package auth

import (
    "context"
    "crypto/rand"
    "fmt"
    "log"
    "time"

    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/mr-tron/base58"
)

// APIToken represents an opaque API token record stored as a unit
type APIToken struct {
    Token      string     `json:"token"`
    OrgID      string     `json:"org_id"`               // Organization UUID
    Subject    string     `json:"sub"`
    Email      string     `json:"email,omitempty"`
    Groups     []string   `json:"groups,omitempty"`
    Scopes     []string   `json:"scopes,omitempty"`
    CreatedAt  time.Time  `json:"created_at"`
    LastUsedAt time.Time  `json:"last_used_at,omitempty"`
    ExpiresAt  *time.Time `json:"expires_at,omitempty"` // nil means never expires
    Status     string     `json:"status"` // active, revoked, expired
}

// APITokenManager issues and verifies opaque tokens for the TFE API surface.
// Tokens are stored via a TokenStore abstraction for flexibility.
type APITokenManager struct {
    store TokenStore // Required - tokens stored via abstraction (blob, db, or external service)
}

// NewAPITokenManager creates a token manager with the given store.
func NewAPITokenManager(store TokenStore) *APITokenManager {
    return &APITokenManager{
        store: store,
    }
}

// Deprecated: Use NewAPITokenManager with NewBlobTokenStore instead.
// Kept for backward compatibility during migration.
func NewAPITokenManagerFromStore(store storage.UnitStore) *APITokenManager {
    return NewAPITokenManager(NewBlobTokenStore(store))
}

// Issue creates a new opaque API token and persists it, or returns existing active token
func (m *APITokenManager) Issue(ctx context.Context, orgID string, subject, email string, groups []string) (string, error) {
	// Check for existing active token for this user first
	existingToken, err := m.findActiveTokenForUser(ctx, orgID, subject, email)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing token: %w", err)
	}
	if existingToken != "" {
		log.Printf("Reusing existing opaque token for user: %s in org: %s", subject, orgID)
		return existingToken, nil
	}

	// No existing token found, create a new one
	token := "otc_tfe_" + randomBase58(32)
	now := time.Now().UTC()
	
	// Set expiration based on TERRAFORM_TOKEN_TTL environment variable
	var expiresAt *time.Time
	if ttlStr := getenv("OPENTACO_TERRAFORM_TOKEN_TTL", ""); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			expTime := now.Add(ttl)
			expiresAt = &expTime
			log.Printf("Creating opaque token with TTL %s (expires at %s)", ttl, expTime.Format(time.RFC3339))
		} else {
			log.Printf("Warning: Invalid TERRAFORM_TOKEN_TTL format '%s', creating token without expiration", ttlStr)
		}
	} else {
		log.Printf("Creating opaque token without expiration (no TTL configured)")
	}
	
	rec := &APIToken{
		Token:     token,
		OrgID:     orgID,
		Subject:   subject,
		Email:     email,
		Groups:    groups,
		Scopes:    []string{"tfe"},
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Status:    "active",
	}
	if err := m.save(ctx, orgID, rec); err != nil {
		return "", err
	}
	log.Printf("Created new opaque token for user: %s in org: %s", subject, orgID)
	return token, nil
}

// Verify checks an opaque token and returns its record if valid
func (m *APITokenManager) Verify(ctx context.Context, orgID string, token string) (*APIToken, error) {
    rec, err := m.load(ctx, orgID, token)
    if err != nil { return nil, err }
    if rec == nil { return nil, fmt.Errorf("not found") }
    if rec.Status != "active" { return nil, fmt.Errorf("revoked") }
    
    // Check if token has expired
    if rec.ExpiresAt != nil && time.Now().UTC().After(*rec.ExpiresAt) {
        // Automatically mark as expired (but don't save to avoid race conditions)
        return nil, fmt.Errorf("expired")
    }
    
    // update last used asynchronously; ignore errors
    now := time.Now().UTC()
    go func() {
        rec.LastUsedAt = now
        _ = m.save(context.Background(), orgID, rec)
    }()
    return rec, nil
}

// Revoke sets the token status to revoked
func (m *APITokenManager) Revoke(ctx context.Context, orgID string, token string) error {
    rec, err := m.load(ctx, orgID, token)
    if err != nil { return err }
    if rec == nil { return storage.ErrNotFound }
    rec.Status = "revoked"
    return m.save(ctx, orgID, rec)
}

func (m *APITokenManager) save(ctx context.Context, orgID string, rec *APIToken) error {
    return m.store.Save(ctx, orgID, rec)
}

func (m *APITokenManager) load(ctx context.Context, orgID string, token string) (*APIToken, error) {
    return m.store.Load(ctx, orgID, token)
}

// findActiveTokenForUser searches for an existing active token for the given user
func (m *APITokenManager) findActiveTokenForUser(ctx context.Context, orgID, subject, email string) (string, error) {
	// List all tokens for this org
	tokens, err := m.store.List(ctx, orgID, "")
	if err != nil {
		return "", fmt.Errorf("failed to list tokens: %w", err)
	}
	
	now := time.Now().UTC()
	for _, rec := range tokens {
		// Check if this token matches the user and is active and not expired
		if rec != nil && rec.Subject == subject && rec.Email == email && rec.Status == "active" {
			// Check if token has expired
			if rec.ExpiresAt != nil && now.After(*rec.ExpiresAt) {
				continue // Skip expired tokens
			}
			return rec.Token, nil
		}
	}
	return "", nil
}

func randomBase58(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return base58.Encode(b)
}


