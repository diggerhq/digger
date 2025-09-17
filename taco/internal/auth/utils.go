package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mr-tron/base58"
)

// getenv returns an environment variable value with a fallback default
func getenv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// randomB58 generates n random bytes and returns them base58 encoded
func randomB58(n int) string {
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        // This should ideally not happen for a secure random number generator
        panic(err)
    }
    return base58.Encode(b)
}

// encryptAESGCM encrypts data using AES-256-GCM and returns base64-encoded result
func encryptAESGCM(data []byte, key []byte) (string, error) {
    // Create AES cipher
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    
    // Use GCM mode for authenticated encryption
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    // Generate nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", err
    }
    
    // Encrypt the data
    ciphertext := gcm.Seal(nonce, nonce, data, nil)
    
    // Return base64-encoded result
    return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decryptAESGCM decrypts base64-encoded AES-256-GCM data
func decryptAESGCM(encryptedData string, key []byte) ([]byte, error) {
    // Decode base64  
    ciphertext, err := base64.RawURLEncoding.DecodeString(encryptedData)
    if err != nil {
        return nil, fmt.Errorf("invalid data format")
    }
    
    // Create AES cipher
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    // Use GCM mode
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("invalid ciphertext length")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    
    // Decrypt
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, fmt.Errorf("decryption failed")
    }
    
    return plaintext, nil
}
