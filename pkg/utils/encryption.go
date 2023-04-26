package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
)

type Encrypt interface {
	EncryptFile(string) error
	DecryptFile(string) error
}

type Encryptor struct {
	Token string
}

func (e *Encryptor) DecryptFile(filename string) error {

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading plan file: %v", err)
	}

	decryptedData, err := decrypt(data, e.Token)
	if err != nil {
		fmt.Errorf("error decrypting plan file: %v", err)
	}

	err = ioutil.WriteFile(filename, decryptedData, 0644)
	if err != nil {
		return fmt.Errorf("error writing decrypted plan file: %v", err)
	}
	return nil
}

func (e *Encryptor) EncryptFile(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading plan file: %v", err)
	}

	encryptedData, err := encrypt(data, e.Token)
	if err != nil {
		return fmt.Errorf("error encrypting plan file: %v", err)
	}

	err = ioutil.WriteFile(filename, encryptedData, 0644)
	if err != nil {
		return fmt.Errorf("error writing encrypted plan file: %v", err)
	}

	return nil
}

func decrypt(data []byte, token string) ([]byte, error) {
	key := createKey(token)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("encrypted data is too short")
	}

	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return data, nil
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
