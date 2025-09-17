package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
)

type mode int

const (
    modeVerify mode = iota
    modeSkip
)

// oidcVerifier implements Verifier. It can either verify with the issuer JWKS,
// or (dev) skip signature verification and only parse claims.
type oidcVerifier struct {
    m        mode
    verifier *gooidc.IDTokenVerifier
}

// NewFromEnv constructs a verifier using environment variables:
// - OPENTACO_AUTH_ISSUER, OPENTACO_AUTH_CLIENT_ID for real verification
// - OPENTACO_AUTH_DEV_SKIP_VERIFY=true to skip verification (dev only)
func NewFromEnv() (Verifier, error) {
    if strings.EqualFold(os.Getenv("OPENTACO_AUTH_DEV_SKIP_VERIFY"), "true") || os.Getenv("OPENTACO_AUTH_DEV_SKIP_VERIFY") == "1" {
        return &oidcVerifier{m: modeSkip}, nil
    }
    issuer := os.Getenv("OPENTACO_AUTH_ISSUER")
    clientID := os.Getenv("OPENTACO_AUTH_CLIENT_ID")
    if issuer == "" || clientID == "" {
        return nil, errors.New("OIDC not configured; set OPENTACO_AUTH_ISSUER and OPENTACO_AUTH_CLIENT_ID or OPENTACO_AUTH_DEV_SKIP_VERIFY=true")
    }
    ctx := context.Background()
    provider, err := gooidc.NewProvider(ctx, issuer)
    if err != nil { return nil, err }
    v := provider.Verifier(&gooidc.Config{ClientID: clientID})
    return &oidcVerifier{m: modeVerify, verifier: v}, nil
}

func (o *oidcVerifier) VerifyIDToken(idToken string) (string, []string, error) {
    switch o.m {
    case modeVerify:
        ctx := context.Background()
        tok, err := o.verifier.Verify(ctx, idToken)
        if err != nil { return "", nil, err }
        var claims struct {
            Sub    string   `json:"sub"`
            Groups []string `json:"groups"`
        }
        _ = tok.Claims(&claims)
        if claims.Sub != "" { return claims.Sub, claims.Groups, nil }
        return tok.Subject, claims.Groups, nil
    case modeSkip:
        parts := strings.Split(idToken, ".")
        if len(parts) < 2 { return "", nil, errors.New("invalid id_token") }
        payload, err := base64.RawURLEncoding.DecodeString(parts[1])
        if err != nil { return "", nil, err }
        var m map[string]any
        if err := json.Unmarshal(payload, &m); err != nil { return "", nil, err }
        sub, _ := m["sub"].(string)
        var groups []string
        if g, ok := m["groups"].([]any); ok {
            for _, v := range g { if s, ok := v.(string); ok { groups = append(groups, s) } }
        }
        if sub == "" { sub = "dev-user" }
        return sub, groups, nil
    default:
        return "", nil, errors.New("invalid verifier mode")
    }
}

