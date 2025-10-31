package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
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