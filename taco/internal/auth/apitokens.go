package auth

import (
    "context"
    "crypto/rand"
    "encoding/json"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/mr-tron/base58"
)

// APIToken represents an opaque API token record stored as a unit
type APIToken struct {
    Token      string     `json:"token"`
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
// Tokens are stored as units with IDs like "_tfe_tokens/TOKEN_VALUE" for persistence.
type APITokenManager struct {
    store storage.UnitStore // Required - tokens stored as units in blob storage
}

func NewAPITokenManagerFromStore(store storage.UnitStore) *APITokenManager {
    return &APITokenManager{
        store: store,
    }
}

// Issue creates a new opaque API token and persists it, or returns existing active token
func (m *APITokenManager) Issue(ctx context.Context, subject, email string, groups []string) (string, error) {
	// Check for existing active token for this user first
	existingToken, err := m.findActiveTokenForUser(ctx, subject, email)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing token: %w", err)
	}
	if existingToken != "" {
		log.Printf("Reusing existing opaque token for user: %s", subject)
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
		Subject:   subject,
		Email:     email,
		Groups:    groups,
		Scopes:    []string{"tfe"},
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Status:    "active",
	}
	if err := m.save(ctx, rec); err != nil {
		return "", err
	}
	log.Printf("Created new opaque token for user: %s", subject)
	return token, nil
}

// Verify checks an opaque token and returns its record if valid
func (m *APITokenManager) Verify(ctx context.Context, token string) (*APIToken, error) {
    rec, err := m.load(ctx, token)
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
        _ = m.save(context.Background(), rec)
    }()
    return rec, nil
}

// Revoke sets the token status to revoked
func (m *APITokenManager) Revoke(ctx context.Context, token string) error {
    rec, err := m.load(ctx, token)
    if err != nil { return err }
    if rec == nil { return storage.ErrNotFound }
    rec.Status = "revoked"
    return m.save(ctx, rec)
}

func (m *APITokenManager) save(ctx context.Context, rec *APIToken) error {
    // Store token as a unit with ID: _tfe_tokens/TOKEN
    unitID := "_tfe_tokens/" + rec.Token
    b, _ := json.Marshal(rec)
    return m.store.Upload(ctx, unitID, b, "")
}

func (m *APITokenManager) load(ctx context.Context, token string) (*APIToken, error) {
    // Load token from unit ID: _tfe_tokens/TOKEN
    unitID := "_tfe_tokens/" + token
    b, err := m.store.Download(ctx, unitID)
    if err != nil { return nil, err }
    var rec APIToken
    if err := json.Unmarshal(b, &rec); err != nil { return nil, err }
    return &rec, nil
}

// findActiveTokenForUser searches for an existing active token for the given user
func (m *APITokenManager) findActiveTokenForUser(ctx context.Context, subject, email string) (string, error) {
	// List all tokens and check each one
	units, err := m.store.List(ctx, "_tfe_tokens/")
	if err != nil {
		return "", fmt.Errorf("failed to list tokens: %w", err)
	}
	
	now := time.Now().UTC()
	for _, unit := range units {
		// Extract token from unit ID: _tfe_tokens/TOKEN
		token := strings.TrimPrefix(unit.ID, "_tfe_tokens/")
		
		// Load the token record
		rec, err := m.load(ctx, token)
		if err != nil {
			continue // Skip tokens we can't load
		}
		
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


