package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

type Encrypt interface {
	EncryptFile(string, string) (string, error)
	DecryptFile()
}

type Encryptor struct {
	Token string
}

func (e *Encryptor) EncryptFile(filename string) (string, error) {
	encryptedFile := filename + ".enc"

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading plan file:", err)
	}

	encryptedData, err := encrypt(data, e.Token)
	if err != nil {
		return "", fmt.Errorf("error encrypting plan file: %v", err)
	}

	encryptedDataB64 := base64.StdEncoding.EncodeToString(encryptedData)
	err = ioutil.WriteFile(encryptedFile, []byte(encryptedDataB64), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing encrypted plan file: %v", err)
	}

	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			fmt.Println("Error removing plan file:", err)
		}
	}(filename)

	return encryptedFile, nil
}

func encrypt(data []byte, token string) ([]byte, error) {
	key := createKey(token)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func createKey(token string) []byte {
	key := make([]byte, 32)
	copy(key, token)

	return key
}
