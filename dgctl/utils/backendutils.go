package utils

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

func SendZipAsJobArtefact(backendUrl string, zipLocation string, jobToken string) (*int, *string, error) {
	u, err := url.Parse(backendUrl)
	if err != nil {
		return nil, nil, err
	}
	u.Path = path.Join(u.Path, "job_artefacts")
	url := u.String()
	filePath := zipLocation

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil, nil, fmt.Errorf("Error opening file:", err)
	}
	defer file.Close()

	// Create a buffer to store our request body as bytes
	var requestBody bytes.Buffer

	// Create a multipart writer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Create a form file writer for our file field
	fileWriter, err := multipartWriter.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Println("Error creating form file:", err)
		return nil, nil, fmt.Errorf("Error creating form file:", err)
	}

	// Copy the file content to the form file writer
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		fmt.Println("Error copying file content:", err)
		return nil, nil, fmt.Errorf("Error copying file content:", err)
	}

	// Close the multipart writer to finalize the request body
	multipartWriter.Close()

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", url, &requestBody)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, nil, fmt.Errorf("Error creating request:", err)
	}

	// Set the content type header
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", jobToken))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return nil, nil, fmt.Errorf("Error sending request:", err)
	}
	defer resp.Body.Close()

	// Read and print the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return nil, nil, fmt.Errorf("Error reading response: %v", err)
	}

	b := string(body)
	return &resp.StatusCode, &b, nil
}
