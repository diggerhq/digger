package utils

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestEncryptingAndDecrypting(t *testing.T) {
	planfile := "plan.out"
	content := "test"
	deleteFile := createFile(planfile, content)
	defer deleteFile()

	token := "test"
	encryptor := Encryptor{Token: token}

	// Act
	err := encryptor.EncryptFile(planfile)
	if err != nil {
		t.Errorf("error encrypting plan file: %v", err)
	}

	fileContent, err := ioutil.ReadFile(planfile)

	if err != nil {
		t.Errorf("error reading encrypted file: %v", err)
	}
	println(string(fileContent))

	assert.NotEqual(t, []byte(content), fileContent)

	// Assert
	// Check that the encrypted file is different from the plan
	err = encryptor.DecryptFile(planfile)
	if err != nil {
		t.Errorf("error decrypting plan file: %v", err)
	}
	// Check that the decrypted file is the same as the plan

	fileContent, err = ioutil.ReadFile(planfile)

	assert.Equal(t, content, string(fileContent))
}

func createFile(filepath string, content string) func() {
	f, err := os.Create(filepath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.WriteString(content)
	if err != nil {
		log.Fatal(err)
	}

	return func() {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}
