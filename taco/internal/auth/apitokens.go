package auth

import (
    "context"
    "crypto/rand"
    "encoding/json"
    "fmt"
    "log"
    "path"
    "strings"
    "sync"
    "time"

    "github.com/diggerhq/digger/opentaco/internal/storage"
    awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/mr-tron/base58"
)

// APIToken represents an opaque API token record stored in S3 or memory
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

// APITokenManager issues and verifies opaque tokens for the TFE API surface
type APITokenManager struct {
    // backing store (optional); if nil, memory fallback
    s3store storage.S3Store
    // memory fallback
    mu     sync.RWMutex
    inmem  map[string]*APIToken
}

func NewAPITokenManagerFromStore(store storage.UnitStore) *APITokenManager {
    var s3s storage.S3Store
    if ss, ok := store.(storage.S3Store); ok {
        s3s = ss
    }
    return &APITokenManager{
        s3store: s3s,
        inmem:   map[string]*APIToken{},
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
    if m.s3store != nil {
        key := m.s3Key(rec.Token)
        b, _ := json.Marshal(rec)
        _, err := m.s3store.GetS3Client().PutObject(ctx, &awsS3.PutObjectInput{
            Bucket: awsString(m.s3store.GetS3Bucket()),
            Key:    awsString(key),
            Body:   strings.NewReader(string(b)),
            ContentType: awsString("application/json"),
            ACL:    types.ObjectCannedACLPrivate,
        })
        return err
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    m.inmem[rec.Token] = rec
    return nil
}

func (m *APITokenManager) load(ctx context.Context, token string) (*APIToken, error) {
    if m.s3store != nil {
        key := m.s3Key(token)
        out, err := m.s3store.GetS3Client().GetObject(ctx, &awsS3.GetObjectInput{
            Bucket: awsString(m.s3store.GetS3Bucket()),
            Key:    awsString(key),
        })
        if err != nil { return nil, err }
        defer out.Body.Close()
        var rec APIToken
        if err := json.NewDecoder(out.Body).Decode(&rec); err != nil { return nil, err }
        return &rec, nil
    }
    m.mu.RLock()
    defer m.mu.RUnlock()
    if rec, ok := m.inmem[token]; ok {
        return rec, nil
    }
    return nil, storage.ErrNotFound
}

// findActiveTokenForUser searches for an existing active token for the given user
func (m *APITokenManager) findActiveTokenForUser(ctx context.Context, subject, email string) (string, error) {
	if m.s3store != nil {
		// S3 implementation - list all tokens and check each one
		return m.findActiveTokenForUserS3(ctx, subject, email)
	}
	
	// Memory implementation - iterate through inmem map
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now().UTC()
	for _, rec := range m.inmem {
		if rec.Subject == subject && rec.Email == email && rec.Status == "active" {
			// Check if token has expired
			if rec.ExpiresAt != nil && now.After(*rec.ExpiresAt) {
				continue // Skip expired tokens
			}
			return rec.Token, nil
		}
	}
	return "", nil
}

func (m *APITokenManager) findActiveTokenForUserS3(ctx context.Context, subject, email string) (string, error) {
	// List all objects in the _tfe_tokens prefix
	prefix := path.Join(m.s3store.GetS3Prefix(), "_tfe_tokens") + "/"
	listInput := &awsS3.ListObjectsV2Input{
		Bucket: awsString(m.s3store.GetS3Bucket()),
		Prefix: awsString(prefix),
	}
	
	paginator := awsS3.NewListObjectsV2Paginator(m.s3store.GetS3Client(), listInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list S3 objects: %w", err)
		}
		
		for _, obj := range page.Contents {
			// Extract token from key: prefix/_tfe_tokens/TOKEN.json -> TOKEN
			key := *obj.Key
			if !strings.HasSuffix(key, ".json") {
				continue
			}
			tokenFromKey := strings.TrimSuffix(path.Base(key), ".json")
			
			// Load the token record
			rec, err := m.load(ctx, tokenFromKey)
			if err != nil {
				continue // Skip tokens we can't load
			}
			
			// Check if this token matches the user and is active and not expired
			if rec != nil && rec.Subject == subject && rec.Email == email && rec.Status == "active" {
				// Check if token has expired
				if rec.ExpiresAt != nil && time.Now().UTC().After(*rec.ExpiresAt) {
					continue // Skip expired tokens
				}
				return rec.Token, nil
			}
		}
	}
	
	return "", nil
}

func (m *APITokenManager) s3Key(token string) string {
	// store under <prefix>_tfe_tokens/<token>.json to avoid collisions with units
	p := m.s3store.GetS3Prefix()
	// path.Join will clean slashes appropriately
	return path.Join(p, "_tfe_tokens", token+".json")
}

func randomBase58(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return base58.Encode(b)
}

func awsString(s string) *string { return &s }


