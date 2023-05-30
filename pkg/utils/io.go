package utils

import (
	"archive/zip"
	"bytes"
	"github.com/bmatcuk/doublestar/v4"
	"io"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"strings"
)

type Zip interface {
	GetFileFromZip(zipFile string, filename string) (string, error)
}

type Zipper struct {
}

func (z *Zipper) GetFileFromZip(zipFile string, filename string) (string, error) {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		if strings.HasSuffix(file.Name, filename) {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			// Create a buffer to write our archive to.
			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, rc)
			if err != nil {
				return "", err
			}

			// Create a temporary file within our temp-images directory that follows
			// a particular naming pattern
			tempFile, err := ioutil.TempFile("", "digger-*.tfplan")
			if err != nil {
				return "", err
			}
			defer tempFile.Close()

			// Read all of the contents of our archive into a byte array.
			contents, err := ioutil.ReadAll(buf)
			if err != nil {
				return "", err
			}

			// Write the contents of the archive, from the byte array, to our temporary file.
			_, err = tempFile.Write(contents)
			if err != nil {
				return "", err
			}

			return tempFile.Name(), nil
		}
	}

	return "", nil
}

func NormalizeFileName(fileName string) string {
	res, err := filepath.Abs(path.Join("/", fileName))
	if err != nil {
		log.Fatalf("Failed to convert path to absolute: %v", err)
	}
	return res
}

func MatchIncludeExcludePatternsToFile(fileToMatch string, includePatterns []string, excludePatterns []string) bool {
	fileToMatch = NormalizeFileName(fileToMatch)
	for i, _ := range includePatterns {
		includePatterns[i] = NormalizeFileName(includePatterns[i])
	}
	for i, _ := range excludePatterns {
		excludePatterns[i] = NormalizeFileName(excludePatterns[i])
	}

	matching := false
	for _, ipattern := range includePatterns {
		isMatched, err := doublestar.PathMatch(ipattern, fileToMatch)
		if err != nil {
			log.Fatalf("Failed to match modified files (%v, %v): Error: %v", fileToMatch, ipattern, err)
		}
		if isMatched {
			matching = true
			break
		}
	}

	for _, epattern := range excludePatterns {
		excluded, err := doublestar.PathMatch(epattern, fileToMatch)
		if err != nil {
			log.Fatalf("Failed to match modified files (%v, %v): Error: %v", fileToMatch, epattern, err)
		}
		if excluded {
			matching = false
			break
		}
	}

	return matching
}
