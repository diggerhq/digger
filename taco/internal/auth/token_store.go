package auth

import (
	"context"
)

// TokenStore defines the interface for persisting API tokens.
// This abstraction allows swapping storage implementations (blob, database, external token service).
type TokenStore interface {
	// Save persists a token for the given organization
	Save(ctx context.Context, orgID string, token *APIToken) error
	
	// Load retrieves a token by its value for the given organization
	Load(ctx context.Context, orgID string, tokenValue string) (*APIToken, error)
	
	// List returns all tokens with the given prefix (for finding user tokens)
	List(ctx context.Context, orgID string, prefix string) ([]*APIToken, error)
	
	// Delete removes a token (for explicit revocation cleanup)
	Delete(ctx context.Context, orgID string, tokenValue string) error
}

