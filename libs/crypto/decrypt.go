package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

func DecryptValueUsingPrivateKey(encryptedDataBase64 string, privateKeyPEM string) (string, error) {
	// Decode the Base64-encoded encrypted data
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedDataBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode Base64 encrypted data: %v", err)
	}

	// Decode the PEM-encoded private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", errors.New("failed to parse PEM block containing the private key")
	}

	// Parse the private key
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}

	// Assert that the key is an RSA private key
	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("not an RSA private key")
	}

	// Decrypt the data
	decryptedData, err := rsa.DecryptPKCS1v15(rand.Reader, rsaPrivateKey, encryptedData)
	if err != nil {
		return "", fmt.Errorf("decryption error: %v", err)
	}

	return string(decryptedData), nil
}
