package spec

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/stretchr/testify/assert"
	"os"
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

func TestDecryptProvider(t *testing.T) {
	// Generate a test key pair
	privateKey, privateKeyPEM, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}

	testCases := []struct {
		variables   []VariableSpec
		plaintext   string
		expectError bool
	}{
		{
			variables: []VariableSpec{{

				Name:     "XYZ",
				Value:    "simple text",
				IsSecret: false,
			}},
			plaintext:   "simple text",
			expectError: false,
		},
		{
			variables: []VariableSpec{{

				Name: "XYZ",
				//Value:    "simple text",
				IsSecret: true,
			}},
			plaintext:   "secret text",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.plaintext, func(t *testing.T) {
			// Encrypt the plaintext
			os.Setenv("DIGGER_PRIVATE_KEY", string(privateKeyPEM))
			if tc.variables[0].IsSecret {
				v, err := rsa.EncryptPKCS1v15(rand.Reader, &privateKey.PublicKey, []byte(tc.plaintext))
				if err != nil {
					t.Fatalf("Failed to encrypt test data: %v", err)
				}
				tc.variables[0].Value = string(v)
			}
			variables, err := VariablesProvider{}.GetVariables(tc.variables)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				value := variables[tc.variables[0].Name]
				assert.Equal(t, tc.plaintext, value)
			}
		})
	}
}
