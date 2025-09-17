package auth

import (
    "context"
    "crypto/rand"
    "encoding/json"
    "fmt"
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
    Token      string    `json:"token"`
    Subject    string    `json:"sub"`
    Email      string    `json:"email,omitempty"`
    Groups     []string  `json:"groups,omitempty"`
    Scopes     []string  `json:"scopes,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
    LastUsedAt time.Time `json:"last_used_at,omitempty"`
    Status     string    `json:"status"` // active, revoked
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

// Issue creates a new opaque API token and persists it
func (m *APITokenManager) Issue(ctx context.Context, subject, email string, groups []string) (string, error) {
    token := "otc_tfe_" + randomBase58(32)
    now := time.Now().UTC()
    rec := &APIToken{
        Token:     token,
        Subject:   subject,
        Email:     email,
        Groups:    groups,
        Scopes:    []string{"tfe"},
        CreatedAt: now,
        Status:    "active",
    }
    if err := m.save(ctx, rec); err != nil {
        return "", err
    }
    return token, nil
}

// Verify checks an opaque token and returns its record if valid
func (m *APITokenManager) Verify(ctx context.Context, token string) (*APIToken, error) {
    rec, err := m.load(ctx, token)
    if err != nil { return nil, err }
    if rec == nil { return nil, fmt.Errorf("not found") }
    if rec.Status != "active" { return nil, fmt.Errorf("revoked") }
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


