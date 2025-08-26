package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the OpenTaco SDK client
type Client struct {
    baseURL    string
    httpClient *http.Client
    authToken  string
}

// NewClient creates a new OpenTaco client
func NewClient(baseURL string) *Client {
    return &Client{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// NewClientWithHTTPClient creates a new client with a custom HTTP client
func NewClientWithHTTPClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// StateMetadata represents state metadata
type StateMetadata struct {
	ID       string    `json:"id"`
	Size     int64     `json:"size"`
	Updated  time.Time `json:"updated"`
	Locked   bool      `json:"locked"`
	LockInfo *LockInfo `json:"lock,omitempty"`
}

// LockInfo represents lock information
type LockInfo struct {
	ID      string    `json:"id"`
	Who     string    `json:"who"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

// CreateStateRequest represents a request to create a state
type CreateStateRequest struct {
	ID string `json:"id"`
}

// CreateStateResponse represents the response from creating a state
type CreateStateResponse struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

// ListStatesResponse represents the response from listing states
type ListStatesResponse struct {
	States []*StateMetadata `json:"states"`
	Count  int              `json:"count"`
}

// CreateState creates a new state
func (c *Client) CreateState(ctx context.Context, stateID string) (*CreateStateResponse, error) {
	req := CreateStateRequest{ID: stateID}
	
	resp, err := c.doJSON(ctx, "POST", "/v1/states", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}

	var result CreateStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListStates lists all states with optional prefix filter
func (c *Client) ListStates(ctx context.Context, prefix string) (*ListStatesResponse, error) {
	path := "/v1/states"
	if prefix != "" {
		path += "?prefix=" + url.QueryEscape(prefix)
	}

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result ListStatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetState gets state metadata
func (c *Client) GetState(ctx context.Context, stateID string) (*StateMetadata, error) {
	encodedID := encodeStateID(stateID)
	resp, err := c.do(ctx, "GET", "/v1/states/"+encodedID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result StateMetadata
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteState deletes a state
func (c *Client) DeleteState(ctx context.Context, stateID string) error {
	encodedID := encodeStateID(stateID)
	resp, err := c.do(ctx, "DELETE", "/v1/states/"+encodedID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return parseError(resp)
	}

	return nil
}

// DownloadState downloads state data
func (c *Client) DownloadState(ctx context.Context, stateID string) ([]byte, error) {
	encodedID := encodeStateID(stateID)
	resp, err := c.do(ctx, "GET", "/v1/states/"+encodedID+"/download", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	return io.ReadAll(resp.Body)
}

// UploadState uploads state data
func (c *Client) UploadState(ctx context.Context, stateID string, data []byte, lockID string) error {
	encodedID := encodeStateID(stateID)
	path := "/v1/states/" + encodedID + "/upload"
	if lockID != "" {
		path += "?if_locked_by=" + url.QueryEscape(lockID)
	}

	resp, err := c.do(ctx, "POST", path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}

	return nil
}

// LockState locks a state
func (c *Client) LockState(ctx context.Context, stateID string, lockInfo *LockInfo) (*LockInfo, error) {
	encodedID := encodeStateID(stateID)
	resp, err := c.doJSON(ctx, "POST", "/v1/states/"+encodedID+"/lock", lockInfo)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result LockInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UnlockState unlocks a state
func (c *Client) UnlockState(ctx context.Context, stateID string, lockID string) error {
	req := map[string]string{"id": lockID}
	encodedID := encodeStateID(stateID)
	
	resp, err := c.doJSON(ctx, "DELETE", "/v1/states/"+encodedID+"/unlock", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}

	return nil
}

// Helper methods

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }

    if c.authToken != "" {
        req.Header.Set("Authorization", "Bearer "+c.authToken)
    }

    return c.httpClient.Do(req)
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(data)
	}

	return c.do(ctx, method, path, body)
}

func parseError(resp *http.Response) error {
	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
	}

	if msg, ok := errResp["error"].(string); ok {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}

	return fmt.Errorf("HTTP %d: %v", resp.StatusCode, errResp)
}

// encodeStateID encodes a state ID for use in URLs by replacing slashes with double underscores
func encodeStateID(id string) string {
    return strings.ReplaceAll(id, "/", "__")
}

// SetBearerToken sets the Authorization bearer token for subsequent requests.
func (c *Client) SetBearerToken(token string) { c.authToken = token }
