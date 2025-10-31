package auth

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/config"
)

// helper: set mock config for each test
func withMockConfig(t *testing.T, secret string, err error, fn func()) {
	orig := config.GetConfig() // capture current provider

	config.SetConfig(&config.MockConfig{
		Secret: secret,
		Err:    err,
	})

	t.Cleanup(func() {
		config.SetConfig(orig)
	})

	fn()
}

func TestSignAndVerify_Success(t *testing.T) {
	withMockConfig(t, "test-secret", nil, func() {
		exp := time.Now().Add(1 * time.Hour)

		signed, err := SignURL("https://example.com", "/files/123", exp)
		if err != nil {
			t.Fatalf("SignURL() unexpected error: %v", err)
		}

		if err := VerifySignedUrl(signed); err != nil {
			t.Fatalf("VerifySignedUrl() unexpected error: %v", err)
		}
	})
}

func TestVerifySignedUrl_Expired(t *testing.T) {
	withMockConfig(t, "test-secret", nil, func() {
		expired := time.Now().Add(-2 * time.Minute)

		signed, err := SignURL("https://example.com", "/files/123", expired)
		if err != nil {
			t.Fatalf("SignURL() unexpected error: %v", err)
		}

		if err := VerifySignedUrl(signed); err == nil {
			t.Fatalf("expected error for expired URL, got nil")
		}
	})
}

func TestVerifySignedUrl_TamperedPath(t *testing.T) {
	withMockConfig(t, "test-secret", nil, func() {
		exp := time.Now().Add(1 * time.Hour)

		signed, err := SignURL("https://example.com", "/files/123", exp)
		if err != nil {
			t.Fatalf("SignURL() unexpected error: %v", err)
		}

		// parse and change the path AFTER signing
		u, _ := url.Parse(signed)
		u.Path = "/files/999" // attacker changes resource
		tampered := u.String()

		if err := VerifySignedUrl(tampered); err == nil {
			t.Fatalf("expected invalid signature error for tampered path, got nil")
		}
	})
}

func TestVerifySignedUrl_TamperedSignature(t *testing.T) {
	withMockConfig(t, "test-secret", nil, func() {
		exp := time.Now().Add(1 * time.Hour)

		signed, err := SignURL("https://example.com", "/files/123", exp)
		if err != nil {
			t.Fatalf("SignURL() unexpected error: %v", err)
		}

		u, _ := url.Parse(signed)
		q := u.Query()
		q.Set("sig", "definitely-wrong-signature")
		u.RawQuery = q.Encode()
		tampered := u.String()

		if err := VerifySignedUrl(tampered); err == nil {
			t.Fatalf("expected invalid signature error for tampered signature, got nil")
		}
	})
}

func TestVerifySignedUrl_BadExpiryFormat(t *testing.T) {
	withMockConfig(t, "test-secret", nil, func() {
		exp := time.Now().Add(1 * time.Hour)

		signed, err := SignURL("https://example.com", "/files/123", exp)
		if err != nil {
			t.Fatalf("SignURL() unexpected error: %v", err)
		}

		u, _ := url.Parse(signed)
		q := u.Query()
		q.Set("exp", "not-a-timestamp") // break exp
		u.RawQuery = q.Encode()
		bad := u.String()

		if err := VerifySignedUrl(bad); err == nil {
			t.Fatalf("expected error for bad exp format, got nil")
		}
	})
}

func TestSignURL_SecretError(t *testing.T) {
	withMockConfig(t, "", errors.New("nope"), func() {
		_, err := SignURL("https://example.com", "/files/123", time.Now().Add(time.Hour))
		if err == nil {
			t.Fatalf("expected error because secret retrieval failed in SignURL, got nil")
		}
	})
}

func TestVerifySignedUrl_SecretError(t *testing.T) {
	withMockConfig(t, "", errors.New("nope"), func() {
		// doesn't matter if URL shape is valid, config will fail first
		testURL := "https://example.com/files/123?exp=123&sig=abc"

		if err := VerifySignedUrl(testURL); err == nil {
			t.Fatalf("expected error because secret retrieval failed in VerifySignedUrl, got nil")
		}
	})
}

