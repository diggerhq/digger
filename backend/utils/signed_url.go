package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

// SignURL creates a signed URL valid until expiry.
// Uses WEBHOOK_SECRET for signing and HOSTNAME as base URL.
// This duplicates the pattern from taco/internal/auth/signed_url.go for use in the backend.
func SignURL(path string, expiry time.Time) (string, error) {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		return "", fmt.Errorf("WEBHOOK_SECRET not configured")
	}

	baseURL := os.Getenv("HOSTNAME")
	if baseURL == "" {
		return "", fmt.Errorf("HOSTNAME not configured")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid HOSTNAME: %w", err)
	}
	u.Path = path
	q := u.Query()
	q.Set("exp", fmt.Sprint(expiry.Unix()))

	// Compute HMAC (same algorithm as taco/internal/auth/signed_url.go)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(path + q.Get("exp")))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	q.Set("sig", sig)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// VerifySignedURL verifies a signed URL and returns error if invalid.
func VerifySignedURL(signedURL string) error {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		return fmt.Errorf("WEBHOOK_SECRET not configured")
	}

	u, err := url.Parse(signedURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	path := u.Path
	query := u.Query()
	exp := query.Get("exp")
	sig := query.Get("sig")

	if exp == "" || sig == "" {
		return fmt.Errorf("missing exp or sig query parameters")
	}

	// Check expiry
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expiry: %w", err)
	}
	if time.Now().Unix() > unix {
		return fmt.Errorf("URL expired")
	}

	// Verify signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(path + exp))
	expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
