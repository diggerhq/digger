package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/diggerhq/digger/backend/models"
)

// Encrypt encrypts a plaintext string using AES-256-GCM
func AESEncrypt(key []byte, plaintext string) (string, error) {
	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to create nonce: %v", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64 encoded string
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded ciphertext using AES-256-GCM
func AESDecrypt(key []byte, encodedCiphertext string) (string, error) {
	// Decode base64 string
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %v", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce size
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Split nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %v", err)
	}

	return string(plaintext), nil
}

// represents a decrypted record
type DecryptedVCSConnection struct {
	GithubId               int64
	ClientID               string
	ClientSecret           string
	WebhookSecret          string
	PrivateKey             string
	PrivateKeyBase64       string
	Org                    string
	Name                   string
	GithubAppUrl           string
	OrganisationID         uint
	BitbucketAccessToken   string
	BitbucketWebhookSecret string
}

func DecryptConnection(g *models.VCSConnection, key []byte) (*DecryptedVCSConnection, error) {
	// Create decrypted version
	decrypted := &DecryptedVCSConnection{
		GithubId:       g.GithubId,
		ClientID:       g.ClientID,
		Org:            g.Org,
		Name:           g.Name,
		GithubAppUrl:   g.GithubAppUrl,
		OrganisationID: g.OrganisationID,
	}

	// Decrypt ClientSecret
	if g.ClientSecretEncrypted != "" {
		clientSecret, err := AESDecrypt(key, g.ClientSecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt client secret: %w", err)
		}
		decrypted.ClientSecret = clientSecret
	}

	// Decrypt WebhookSecret
	if g.WebhookSecretEncrypted != "" {
		webhookSecret, err := AESDecrypt(key, g.WebhookSecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt webhook secret: %w", err)
		}
		decrypted.WebhookSecret = webhookSecret
	}

	// Decrypt PrivateKey
	if g.PrivateKeyEncrypted != "" {
		privateKey, err := AESDecrypt(key, g.PrivateKeyEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key: %w", err)
		}
		decrypted.PrivateKey = privateKey
	}

	// Decrypt PrivateKeyBase64
	if g.PrivateKeyBase64Encrypted != "" {
		privateKeyBase64, err := AESDecrypt(key, g.PrivateKeyBase64Encrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key base64: %w", err)
		}
		decrypted.PrivateKeyBase64 = privateKeyBase64
	}

	if g.BitbucketAccessTokenEncrypted != "" {
		bitbucketAccessToken, err := AESDecrypt(key, g.BitbucketAccessTokenEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key base64: %w", err)
		}
		decrypted.BitbucketAccessToken = bitbucketAccessToken
	}

	if g.BitbucketWebhookSecretEncrypted != "" {
		bitbucketWebhookSecret, err := AESDecrypt(key, g.BitbucketWebhookSecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key base64: %w", err)
		}
		decrypted.BitbucketWebhookSecret = bitbucketWebhookSecret
	}

	return decrypted, nil
}
