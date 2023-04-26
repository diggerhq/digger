package utils

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
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
	println(zipFile)
	println(len(reader.File))
	println(filename)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		println(file.Name)

		if file.Name == filename {
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
