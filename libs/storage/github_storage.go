package storage

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ArtifactClient interface defines the contract for artifact operations
type ArtifactClient interface {
	CreateArtifact(ctx context.Context, req *CreateArtifactRequest) (*CreateArtifactResponse, error)
	FinalizeArtifact(ctx context.Context, req *FinalizeArtifactRequest) (*FinalizeArtifactResponse, error)
}

// artifactClientImpl implements the ArtifactClient interface
type artifactClientImpl struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// artifactClientConfig holds the configuration for the artifact client
type artifactClientConfig struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// newArtifactClient creates a new instance of the artifact client
func newArtifactClient() ArtifactClient {
	// Get the API URL from environment or use default
	baseURL := os.Getenv("ACTIONS_RUNTIME_URL")
	if baseURL == "" {
		baseURL = "https://api.github.com" // Default URL, adjust as needed
	}

	// Get the authentication token from environment
	token := os.Getenv("ACTIONS_RUNTIME_TOKEN")
	if token == "" {
		// In a real implementation, you might want to handle this error differently
		panic("ACTIONS_RUNTIME_TOKEN environment variable is required")
	}

	return &artifactClientImpl{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// CreateArtifact sends a request to create a new artifact
func (c *artifactClientImpl) CreateArtifact(ctx context.Context, req *CreateArtifactRequest) (*CreateArtifactResponse, error) {
	endpoint := fmt.Sprintf("%s/artifacts/create", c.baseURL)
	resp := &CreateArtifactResponse{}
	err := c.sendRequest(ctx, "POST", endpoint, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// FinalizeArtifact sends a request to finalize an artifact upload
func (c *artifactClientImpl) FinalizeArtifact(ctx context.Context, req *FinalizeArtifactRequest) (*FinalizeArtifactResponse, error) {
	endpoint := fmt.Sprintf("%s/artifacts/finalize", c.baseURL)
	resp := &FinalizeArtifactResponse{}
	err := c.sendRequest(ctx, "POST", endpoint, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// sendRequest is a helper method to send HTTP requests
func (c *artifactClientImpl) sendRequest(ctx context.Context, method, url string, reqBody interface{}, respBody interface{}) error {
	// Marshal request body
	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Unmarshal response
	if err := json.Unmarshal(body, respBody); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// For testing purposes, you might want to add this constructor
func newArtifactClientWithConfig(config artifactClientConfig) ArtifactClient {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}

	return &artifactClientImpl{
		baseURL:    config.BaseURL,
		token:      config.Token,
		httpClient: config.HTTPClient,
	}
}

// UploadArtifactOptions represents optional parameters for artifact upload
type UploadArtifactOptions struct {
	RetentionDays    *int
	CompressionLevel *int
}

// UploadArtifactResponse represents the response from an artifact upload
type UploadArtifactResponse struct {
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	ID     int    `json:"id"`
}

// UploadArtifact uploads files as an artifact
func UploadArtifact(ctx context.Context, name string, files []string, rootDirectory string, options *UploadArtifactOptions) (*UploadArtifactResponse, error) {
	// Validate inputs
	if err := validateArtifactName(name); err != nil {
		return nil, err
	}
	if err := validateRootDirectory(rootDirectory); err != nil {
		return nil, err
	}

	// Get zip specification
	zipSpec, err := getUploadZipSpecification(files, rootDirectory)
	if err != nil {
		return nil, err
	}
	if len(zipSpec) == 0 {
		return nil, &FilesNotFoundError{Files: files}
	}

	// Get backend IDs
	backendIDs, err := GetBackendIdsFromToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get backend IDs: %w", err)
	}

	// Create artifact client
	client := newArtifactClient()

	// Create artifact request
	createReq := &CreateArtifactRequest{
		WorkflowRunBackendID:    backendIDs.WorkflowRunBackendId,
		WorkflowJobRunBackendID: backendIDs.WorkflowJobRunBackendId,
		Name:                    name,
		Version:                 4,
	}

	// Add expiration if retention days specified
	if options != nil && options.RetentionDays != nil {
		expiresAt, err := getExpiration(options.RetentionDays)
		if err != nil {
			return nil, err
		}
		createReq.ExpiresAt = expiresAt
	}

	// Create artifact
	createResp, err := client.CreateArtifact(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact: %w", err)
	}
	if !createResp.OK {
		return nil, &InvalidResponseError{Message: "CreateArtifact: response from backend was not ok"}
	}

	// Create zip upload stream
	compressionLevel := 0
	if options != nil && options.CompressionLevel != nil {
		compressionLevel = *options.CompressionLevel
	}
	zipStream, err := createZipUploadStream(zipSpec, compressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create zip stream: %w", err)
	}

	// Upload to blob storage
	uploadResult, err := uploadZipToBlobStorage(ctx, createResp.SignedUploadURL, zipStream)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to blob storage: %w", err)
	}

	// Finalize artifact
	finalizeReq := &FinalizeArtifactRequest{
		WorkflowRunBackendID:    backendIDs.WorkflowRunBackendId,
		WorkflowJobRunBackendID: backendIDs.WorkflowJobRunBackendId,
		Name:                    name,
		Size:                    fmt.Sprintf("%d", uploadResult.UploadSize),
	}

	if uploadResult.SHA256Hash != "" {
		finalizeReq.Hash = &StringValue{
			Value: fmt.Sprintf("sha256:%s", uploadResult.SHA256Hash),
		}
	}

	log.Printf("Finalizing artifact upload")

	finalizeResp, err := client.FinalizeArtifact(ctx, finalizeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize artifact: %w", err)
	}
	if !finalizeResp.OK {
		return nil, &InvalidResponseError{Message: "FinalizeArtifact: response from backend was not ok"}
	}

	artifactID := new(big.Int)
	artifactID.SetString(finalizeResp.ArtifactID, 10)

	log.Printf("Artifact %s.zip successfully finalized. Artifact ID %s", name, artifactID.String())

	return &UploadArtifactResponse{
		Size:   uploadResult.UploadSize,
		Digest: uploadResult.SHA256Hash,
		ID:     int(artifactID.Int64()),
	}, nil
}

// Error types
type FilesNotFoundError struct {
	Files []string
}

func (e *FilesNotFoundError) Error() string {
	return fmt.Sprintf("no files were found to upload: %v", e.Files)
}

type InvalidResponseError struct {
	Message string
}

func (e *InvalidResponseError) Error() string {
	return e.Message
}

// Request/Response types
type CreateArtifactRequest struct {
	WorkflowRunBackendID    string     `json:"workflow_run_backend_id"`
	WorkflowJobRunBackendID string     `json:"workflow_job_run_backend_id"`
	Name                    string     `json:"name"`
	Version                 int        `json:"version"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
}

type CreateArtifactResponse struct {
	OK              bool   `json:"ok"`
	SignedUploadURL string `json:"signed_upload_url"`
}

type FinalizeArtifactRequest struct {
	WorkflowRunBackendID    string       `json:"workflow_run_backend_id"`
	WorkflowJobRunBackendID string       `json:"workflow_job_run_backend_id"`
	Name                    string       `json:"name"`
	Size                    string       `json:"size"`
	Hash                    *StringValue `json:"hash,omitempty"`
}

type FinalizeArtifactResponse struct {
	OK         bool   `json:"ok"`
	ArtifactID string `json:"artifact_id"`
}

type StringValue struct {
	Value string `json:"value"`
}

type BackendIDs struct {
	WorkflowRunBackendID    string
	WorkflowJobRunBackendID string
}

type UploadZipSpecification struct {
	SourcePath string
	TargetPath string
}

type UploadResult struct {
	UploadSize int64
	SHA256Hash string
}

const (
	// Maximum artifact name length
	maxArtifactNameLength = 256
)

var (
	// Artifact name validation pattern
	artifactNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
)

func validateArtifactName(name string) error {
	if name == "" {
		return fmt.Errorf("artifact name is required")
	}

	if len(name) > maxArtifactNameLength {
		return fmt.Errorf("artifact name is too long. Maximum length is %d characters", maxArtifactNameLength)
	}

	if !artifactNamePattern.MatchString(name) {
		return fmt.Errorf("artifact name contains invalid characters. Only alphanumeric characters, '_', '-', and '.' are allowed")
	}

	return nil
}

func validateRootDirectory(rootDirectory string) error {
	if rootDirectory == "" {
		return fmt.Errorf("root directory is required")
	}

	info, err := os.Stat(rootDirectory)
	if err != nil {
		return fmt.Errorf("root directory does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("root directory must be a directory")
	}

	return nil
}

func getUploadZipSpecification(files []string, rootDirectory string) ([]UploadZipSpecification, error) {
	var specs []UploadZipSpecification

	for _, file := range files {
		absPath, err := filepath.Abs(filepath.Join(rootDirectory, file))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path: %w", err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			continue // Skip files that don't exist or can't be accessed
		}

		if info.IsDir() {
			// Handle directory
			err := filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					relPath, err := filepath.Rel(rootDirectory, path)
					if err != nil {
						return err
					}
					specs = append(specs, UploadZipSpecification{
						SourcePath: path,
						TargetPath: relPath,
					})
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to walk directory: %w", err)
			}
		} else {
			// Handle single file
			relPath, err := filepath.Rel(rootDirectory, absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to get relative path: %w", err)
			}
			specs = append(specs, UploadZipSpecification{
				SourcePath: absPath,
				TargetPath: relPath,
			})
		}
	}

	return specs, nil
}

type BackendIds struct {
	WorkflowRunBackendId    string
	WorkflowJobRunBackendId string
}

var InvalidJwtError = fmt.Errorf("failed to get backend IDs: the provided JWT token is invalid and/or missing claims")

func GetRuntimeToken() (string, error) {
	token := os.Getenv("ACTIONS_RUNTIME_TOKEN")
	if token == "" {
		return "", fmt.Errorf("unable to get the ACTIONS_RUNTIME_TOKEN env variable")
	}

	// Basic JWT format validation (should have 3 parts separated by dots)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("the provided JWT token is invalid: incorrect format")
	}

	return token, nil
}

// GetBackendIdsFromToken uses the JWT token claims to get the
// workflow run and workflow job run backend ids
func GetBackendIdsFromToken() (BackendIds, error) {
	runtimeToken, err := GetRuntimeToken() // You'll need to implement this similar to the TypeScript version
	if err != nil {
		return BackendIds{}, fmt.Errorf("missing runtime token, %v", err)
	}

	type ActionsToken struct {
		jwt.StandardClaims
		Scp string `json:"scp"`
	}
	// Parse and validate the token
	//var token ActionsToken
	parser := jwt.Parser{
		SkipClaimsValidation: true,
	}

	claims := &ActionsToken{}
	_, _, err = parser.ParseUnverified(runtimeToken, claims)
	if err != nil {
		log.Printf("could not parse token: %v", err)
		return BackendIds{}, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims.Scp == "" {
		log.Printf("scp is empty")
		return BackendIds{}, InvalidJwtError
	}

	// Split the scopes
	scpParts := strings.Split(claims.Scp, " ")
	log.Printf("scp parts are: %v", scpParts)
	if len(scpParts) == 0 {
		log.Printf("no scp parts: %v", scpParts)
		return BackendIds{}, InvalidJwtError
	}

	// Rest of your logic remains the same
	for _, scopes := range scpParts {
		scopeParts := strings.Split(scopes, ":")
		if scopeParts[0] != "Actions.Results" {
			continue
		}

		if len(scopeParts) != 3 {
			return BackendIds{}, InvalidJwtError
		}

		ids := &BackendIds{
			WorkflowRunBackendId:    scopeParts[1],
			WorkflowJobRunBackendId: scopeParts[2],
		}

		log.Printf("Workflow Run Backend ID: %s", ids.WorkflowRunBackendId)
		log.Printf("Workflow Job Run Backend ID: %s", ids.WorkflowJobRunBackendId)

		return BackendIds{}, nil
	}

	return BackendIds{}, InvalidJwtError
}

func createZipUploadStream(specs []UploadZipSpecification, compressionLevel int) (io.Reader, error) {
	buf := new(bytes.Buffer)
	writer := zip.NewWriter(buf)

	if compressionLevel != 0 {
		writer.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, compressionLevel)
		})
	}

	for _, spec := range specs {
		f, err := os.Open(spec.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", spec.SourcePath, err)
		}
		defer f.Close()

		zipFile, err := writer.Create(spec.TargetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry %s: %w", spec.TargetPath, err)
		}

		if _, err := io.Copy(zipFile, f); err != nil {
			return nil, fmt.Errorf("failed to write file content to zip: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf, nil
}

func uploadZipToBlobStorage(ctx context.Context, signedURL string, content io.Reader) (*UploadResult, error) {
	// Create HTTP client
	client := &http.Client{}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "PUT", signedURL, content)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/zip")

	// Calculate SHA256 hash and size while uploading
	hash := sha256.New()
	// Send request
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to upload content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	// Get content length
	contentLength := resp.Request.ContentLength

	return &UploadResult{
		UploadSize: contentLength,
		SHA256Hash: fmt.Sprintf("%x", hash.Sum(nil)),
	}, nil
}

func getExpiration(retentionDays *int) (*time.Time, error) {
	if retentionDays == nil {
		return nil, nil
	}

	if *retentionDays < 1 {
		return nil, fmt.Errorf("retention days must be greater than 0")
	}

	expiresAt := time.Now().AddDate(0, 0, *retentionDays)
	return &expiresAt, nil
}
