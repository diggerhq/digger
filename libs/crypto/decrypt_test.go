package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// generateTestKeyPair generates a test RSA key pair
func generateTestKeyPair() (*rsa.PrivateKey, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return privateKey, privateKeyPEM, nil
}

func TestDecryptWithPrivateKey(t *testing.T) {
	// Generate a test key pair
	privateKey, privateKeyPEM, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}

	testCases := []struct {
		name        string
		plaintext   []byte
		expectError bool
	}{
		{
			name:        "Simple text",
			plaintext:   []byte("Hello, World!"),
			expectError: false,
		},
		{
			name:        "Empty message",
			plaintext:   []byte(""),
			expectError: false,
		},
		{
			name:        "Long message",
			plaintext:   bytes.Repeat([]byte("A"), 100),
			expectError: false,
		},
		{
			name:        "Message with special characters",
			plaintext:   []byte("!@#$%^&*()_+{}:|<>?"),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt the plaintext
			ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &privateKey.PublicKey, tc.plaintext)
			if err != nil {
				t.Fatalf("Failed to encrypt test data: %v", err)
			}

			// Decrypt using our function
			decrypted, err := DecryptValueUsingPrivateKey(ciphertext, string(privateKeyPEM))

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !bytes.Equal(decrypted, tc.plaintext) {
					t.Errorf("Decrypted text doesn't match original. Got %s, want %s", decrypted, tc.plaintext)
				}
			}
		})
	}
}

func TestDecryptWithPrivateKeyInvalidInput(t *testing.T) {
	_, privateKeyPEM, _ := generateTestKeyPair()

	testCases := []struct {
		name           string
		privateKeyPEM  string
		encryptedData  []byte
		expectedErrMsg string
	}{
		{
			name:           "Invalid private key",
			privateKeyPEM:  "invalid key",
			encryptedData:  []byte("some data"),
			expectedErrMsg: "failed to parse PEM block containing the private key",
		},
		{
			name:           "Invalid encrypted data",
			privateKeyPEM:  string(privateKeyPEM),
			encryptedData:  []byte("too short"),
			expectedErrMsg: "decryption error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecryptValueUsingPrivateKey(tc.encryptedData, tc.privateKeyPEM)
			if err == nil {
				t.Errorf("Expected an error, but got none")
			} else if !bytes.Contains([]byte(err.Error()), []byte(tc.expectedErrMsg)) {
				t.Errorf("Expected error message containing '%s', but got: %v", tc.expectedErrMsg, err)
			}
		})
	}
}
