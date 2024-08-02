package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func DecryptValueUsingPrivateKey(encryptedData []byte, privateKeyPEM string) ([]byte, error) {
	// Decode the PEM-encoded private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the private key")
	}

	// Parse the private key
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Assert that the key is an RSA private key
	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}

	// Decrypt the data
	decryptedData, err := rsa.DecryptPKCS1v15(rand.Reader, rsaPrivateKey, encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decryption error: %v", err)
	}

	return decryptedData, nil
}
