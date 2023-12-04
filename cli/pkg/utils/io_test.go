package utils

import (
	"archive/zip"
	"bytes"
	configuration "github.com/diggerhq/digger/libs/digger_config"
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
	zipper := &Zipper{}
	retrievedFile, err := zipper.GetFileFromZip(zipFileName, "file1.txt")
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

	zipper := &Zipper{}
	retrievedFile, err := zipper.GetFileFromZip(zipFileName, "file3.txt")
	if err != nil {
		t.Fatal(err)
	}

	assert.Empty(t, retrievedFile)

	// Clean up the test files.
	os.Remove(zipFileName)
}

func TestNormalizeFileName(t *testing.T) {
	type NormTest struct {
		input, expectedOutput string // input and expected normalized output
	}
	var normTests = []NormTest{
		{"my/directory", "/my/directory"},
		{"./my/directory", "/my/directory"},
	}
	for _, tt := range normTests {
		res := configuration.NormalizeFileName(tt.input)
		assert.Equal(t, tt.expectedOutput, res)
	}
}

func TestMatchIncludeExcludePatternsToFile(t *testing.T) {
	type MatchTest struct {
		fileToMatch     string
		includePatterns []string
		excludePatterns []string
		expectedResult  bool
	}
	var normTests = []MatchTest{
		{"dev/main.tf", []string{"dev/**"}, []string{}, true},
		{"dev/main.tf", []string{"./dev/**"}, []string{}, true},
		{"dev/main.tf", []string{"dev/**"}, []string{"dev/"}, true},
		{"dev/main.tf", []string{"prod/**"}, []string{"dev/"}, false},
		{"modules/moduleA/main.tf", []string{"dev**", "modules/**"}, []string{"dev/"}, true},
		{"modules/moduleA/main.tf", []string{"dev**", "./modules/**"}, []string{"dev/"}, true},
		{"modules/moduleA/main.tf", []string{"dev**"}, []string{"dev/"}, false},
		{"modules/moduleA/main.tf", []string{"dev**", "modules/**"}, []string{"modules/moduleA/**"}, false},
		{"modules/moduleA/main.tf", []string{""}, []string{""}, false},
	}
	for _, tt := range normTests {
		res := configuration.MatchIncludeExcludePatternsToFile(tt.fileToMatch, tt.includePatterns, tt.excludePatterns)
		assert.Equal(t, tt.expectedResult, res)
	}
}
