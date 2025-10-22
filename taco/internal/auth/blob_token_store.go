package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// BlobTokenStore implements TokenStore using blob storage.
// Tokens are stored as blob objects with paths: {orgID}/_tfe_tokens/{tokenValue}
type BlobTokenStore struct {
	store storage.UnitStore
}

// NewBlobTokenStore creates a token store backed by blob storage.
func NewBlobTokenStore(store storage.UnitStore) TokenStore {
	return &BlobTokenStore{store: store}
}

func (b *BlobTokenStore) Save(ctx context.Context, orgID string, token *APIToken) error {
	// Store token as: {orgID}/_tfe_tokens/{tokenValue}
	blobPath := b.tokenPath(orgID, token.Token)
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	return b.store.Upload(ctx, blobPath, data, "")
}

func (b *BlobTokenStore) Load(ctx context.Context, orgID string, tokenValue string) (*APIToken, error) {
	blobPath := b.tokenPath(orgID, tokenValue)
	data, err := b.store.Download(ctx, blobPath)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, nil // Not found, return nil token
		}
		return nil, fmt.Errorf("failed to download token: %w", err)
	}
	
	var token APIToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}
	return &token, nil
}

func (b *BlobTokenStore) List(ctx context.Context, orgID string, prefix string) ([]*APIToken, error) {
	// List all tokens for this org
	listPrefix := fmt.Sprintf("%s/_tfe_tokens/", orgID)
	if prefix != "" {
		listPrefix = fmt.Sprintf("%s/_tfe_tokens/%s", orgID, prefix)
	}
	
	units, err := b.store.List(ctx, listPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}
	
	tokens := make([]*APIToken, 0, len(units))
	for _, unit := range units {
		// Extract token value from path: {orgID}/_tfe_tokens/{tokenValue}
		parts := strings.Split(unit.ID, "/_tfe_tokens/")
		if len(parts) != 2 {
			continue // Skip malformed paths
		}
		tokenValue := parts[1]
		
		// Load the full token
		token, err := b.Load(ctx, orgID, tokenValue)
		if err != nil || token == nil {
			continue // Skip tokens we can't load
		}
		tokens = append(tokens, token)
	}
	
	return tokens, nil
}

func (b *BlobTokenStore) Delete(ctx context.Context, orgID string, tokenValue string) error {
	blobPath := b.tokenPath(orgID, tokenValue)
	return b.store.Delete(ctx, blobPath)
}

// tokenPath generates the blob storage path for a token
func (b *BlobTokenStore) tokenPath(orgID string, tokenValue string) string {
	return fmt.Sprintf("%s/_tfe_tokens/%s", orgID, tokenValue)
}

