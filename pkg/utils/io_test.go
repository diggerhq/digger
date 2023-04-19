package utils

import (
	"archive/zip"
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestGetFileFromZip(t *testing.T) {
	buf := new(bytes.Buffer)

	zipWriter := zip.NewWriter(buf)

	fileNames := []string{"file1.txt", "file2.txt"}
	for _, fileName := range fileNames {
		_, err := zipWriter.Create(fileName)
		if err != nil {
			t.Fatal(err)
		}
	}
	err := zipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	zipFileName := "test.zip"
	err = ioutil.WriteFile(zipFileName, buf.Bytes(), 0644)
	if err != nil {
		t.Fatal(err)
	}

	retrievedFile, err := GetFileFromZip(zipFileName, "file1.txt")
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, retrievedFile)

	// Clean up the test files.
	os.Remove(zipFileName)
}

func TestGetFileFromZipNoFileExists(t *testing.T) {
	buf := new(bytes.Buffer)

	zipWriter := zip.NewWriter(buf)

	fileNames := []string{"file1.txt", "file2.txt"}
	for _, fileName := range fileNames {
		_, err := zipWriter.Create(fileName)
		if err != nil {
			t.Fatal(err)
		}
	}
	err := zipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	zipFileName := "test.zip"
	err = ioutil.WriteFile(zipFileName, buf.Bytes(), 0644)
	if err != nil {
		t.Fatal(err)
	}

	retrievedFile, err := GetFileFromZip(zipFileName, "file3.txt")
	if err != nil {
		t.Fatal(err)
	}

	assert.Empty(t, retrievedFile)

	// Clean up the test files.
	os.Remove(zipFileName)
}
