package oidc

// Package oidc will implement OIDC Relying Party flows (PKCE + optional Device Code)
// and verification helpers for ID Tokens.

// Verifier verifies ID tokens from the configured issuer.
type Verifier interface {
    VerifyIDToken(idToken string) (subject string, groups []string, err error)
}

