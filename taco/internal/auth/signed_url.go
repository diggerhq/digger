package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/config"
)


// SignURL creates a signed URL valid until expiry
func SignURL(baseURL, path string, expiry time.Time) (string, error) {
	secret, err := config.GetConfig().GetSecretKey()
	if err != nil {
		return "", fmt.Errorf("signing secret key error: %w", err)
	}
	u, _ := url.Parse(baseURL)
	u.Path = path
	q := u.Query()
	q.Set("exp", fmt.Sprint(expiry.Unix()))

	// Compute HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(path + q.Get("exp")))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	q.Set("sig", sig)
	u.RawQuery = q.Encode()

	return u.String(), nil
}


func VerifySignedUrl(signedUrl string) error {
	secret, err := config.GetConfig().GetSecretKey()
	if err != nil {
		return fmt.Errorf("signing secret key error: %w", err)
	}

	u, _ := url.Parse(signedUrl)
	path := u.Path
	query := u.Query()
	exp := query.Get("exp")
	sig := query.Get("sig")

	// Parse expiry timestamp
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return fmt.Errorf("signing expired url error: %w", err)
	}
	if time.Now().Unix() > unix {
		return fmt.Errorf("the signed url is expired")
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(path + exp))
	expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return fmt.Errorf("the signed url is invalid")
	}
	return nil
}

// GenerateLogStreamToken creates a time-limited token for log streaming
// Format: {expiry-unix}.{base64-hmac-signature}
// This is designed to be embedded in URL paths (not query strings) since
// Terraform CLI preserves paths but strips/replaces query parameters
func GenerateLogStreamToken(planID string, validFor time.Duration) (string, error) {
	secret, err := config.GetConfig().GetSecretKey()
	if err != nil {
		return "", fmt.Errorf("failed to get secret key: %w", err)
	}

	expiry := time.Now().Add(validFor).Unix()
	expiryStr := strconv.FormatInt(expiry, 10)

	// Compute HMAC: HMAC(planID + expiry)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(planID + expiryStr))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	// Token format: expiry.signature (URL-safe, no special chars)
	token := expiryStr + "." + sig
	return token, nil
}

// VerifyLogStreamToken verifies a log streaming token for a specific plan
// Token format: {expiry}.{signature} (embedded in URL path, not query string)
func VerifyLogStreamToken(token string, planID string) bool {
	secret, err := config.GetConfig().GetSecretKey()
	if err != nil {
		fmt.Printf("[VerifyLogStreamToken] Failed to get secret key: %v\n", err)
		return false
	}

	// Parse token: expiry.signature (use strings.Split - simpler!)
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		fmt.Printf("[VerifyLogStreamToken] Invalid token format (expected expiry.signature): %s\n", token)
		return false
	}
	
	expiryStr := parts[0]
	sig := parts[1]

	fmt.Printf("[VerifyLogStreamToken] planID=%s, expiry=%s, sig=%s...\n", planID, expiryStr, sig[:min(20, len(sig))])

	// Check expiry
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		fmt.Printf("[VerifyLogStreamToken] Failed to parse expiry: %v\n", err)
		return false
	}
	
	now := time.Now().Unix()
	if now > expiry {
		fmt.Printf("[VerifyLogStreamToken] Token expired (now=%d, expiry=%d, diff=%d sec)\n", now, expiry, now-expiry)
		return false
	}

	// Verify signature using same HMAC logic as generation
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(planID + expiryStr))
	expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	isValid := hmac.Equal([]byte(sig), []byte(expectedSig))
	if !isValid {
		fmt.Printf("[VerifyLogStreamToken] SIGNATURE MISMATCH - expected=%s..., got=%s...\n", 
			expectedSig[:min(20, len(expectedSig))], sig[:min(20, len(sig))])
	} else {
		fmt.Printf("[VerifyLogStreamToken] ✓ Token valid for planID=%s\n", planID)
	}
	
	return isValid
}