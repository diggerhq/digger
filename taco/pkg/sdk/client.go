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

// UnitMetadata represents unit metadata
type UnitMetadata struct {
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

// Version represents a version of a unit
type Version struct {
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
}

// CreateUnitRequest represents a request to create a unit
type CreateUnitRequest struct {
	Name string `json:"name"`
}

// CreateUnitResponse represents the response from creating a unit
type CreateUnitResponse struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

// ListUnitsResponse represents the response from listing units
type ListUnitsResponse struct {
	Units []*UnitMetadata `json:"units"`
	Count int             `json:"count"`
}

// ListVersionsResponse represents the response from listing versions
type ListVersionsResponse struct {
	UnitID   string     `json:"unit_id"`
	Versions []*Version `json:"versions"`
	Count    int        `json:"count"`
}

// RestoreVersionRequest represents a request to restore a version
type RestoreVersionRequest struct {
	Timestamp time.Time `json:"timestamp"`
	LockID    string    `json:"lock_id,omitempty"`
}

// RestoreVersionResponse represents the response from restoring a version
type RestoreVersionResponse struct {
	UnitID    string    `json:"unit_id"`
	Timestamp time.Time `json:"restored_timestamp"`
	Message   string    `json:"message"`
}

// UnitStatus represents the dependency status API response
type UnitStatus struct {
	UnitID   string         `json:"unit_id"`
	Status   string         `json:"status"`
	Incoming []IncomingEdge `json:"incoming"`
	Summary  Summary        `json:"summary"`
}

type IncomingEdge struct {
	EdgeID     string `json:"edge_id,omitempty"`
	FromUnitID string `json:"from_unit_id"`
	FromOutput string `json:"from_output"`
	Status     string `json:"status"`
	InDigest   string `json:"in_digest,omitempty"`
	OutDigest  string `json:"out_digest,omitempty"`
	LastInAt   string `json:"last_in_at,omitempty"`
	LastOutAt  string `json:"last_out_at,omitempty"`
}

type Summary struct {
	IncomingOK      int `json:"incoming_ok"`
	IncomingPending int `json:"incoming_pending"`
	IncomingUnknown int `json:"incoming_unknown"`
}

// CreateUnit creates a new unit
func (c *Client) CreateUnit(ctx context.Context, unitID string) (*CreateUnitResponse, error) {
	req := CreateUnitRequest{Name: unitID}

	resp, err := c.doJSON(ctx, "POST", "/v1/units", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}

	var result CreateUnitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListUnits lists all units with optional prefix filter
func (c *Client) ListUnits(ctx context.Context, prefix string) (*ListUnitsResponse, error) {
	path := "/v1/units"
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

	var result ListUnitsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetUnit gets unit metadata
func (c *Client) GetUnit(ctx context.Context, unitID string) (*UnitMetadata, error) {
	encodedID := encodeUnitID(unitID)
	resp, err := c.do(ctx, "GET", "/v1/units/"+encodedID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result UnitMetadata
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteUnit deletes a unit
func (c *Client) DeleteUnit(ctx context.Context, unitID string) error {
	encodedID := encodeUnitID(unitID)
	resp, err := c.do(ctx, "DELETE", "/v1/units/"+encodedID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return parseError(resp)
	}

	return nil
}

// DownloadUnit downloads unit data
func (c *Client) DownloadUnit(ctx context.Context, unitID string) ([]byte, error) {
	encodedID := encodeUnitID(unitID)
	resp, err := c.do(ctx, "GET", "/v1/units/"+encodedID+"/download", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	return io.ReadAll(resp.Body)
}

// UploadUnit uploads unit data
func (c *Client) UploadUnit(ctx context.Context, unitID string, data []byte, lockID string) error {
	encodedID := encodeUnitID(unitID)
	path := "/v1/units/" + encodedID + "/upload"
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

// LockUnit locks a unit
func (c *Client) LockUnit(ctx context.Context, unitID string, lockInfo *LockInfo) (*LockInfo, error) {
	encodedID := encodeUnitID(unitID)
	resp, err := c.doJSON(ctx, "POST", "/v1/units/"+encodedID+"/lock", lockInfo)
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

// UnlockUnit unlocks a unit
func (c *Client) UnlockUnit(ctx context.Context, unitID string, lockID string) error {
	req := map[string]string{"id": lockID}
	encodedID := encodeUnitID(unitID)

	resp, err := c.doJSON(ctx, "DELETE", "/v1/units/"+encodedID+"/unlock", req)
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

// ListUnitVersions lists all versions for a unit
func (c *Client) ListUnitVersions(ctx context.Context, unitID string) ([]*Version, error) {
	encodedID := encodeUnitID(unitID)
	path := fmt.Sprintf("/v1/units/%s/versions", encodedID)

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result ListVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Versions, nil
}

// RestoreUnitVersion restores a unit to a specific version
func (c *Client) RestoreUnitVersion(ctx context.Context, unitID string, versionTimestamp time.Time, lockID string) error {
	encodedID := encodeUnitID(unitID)
	path := fmt.Sprintf("/v1/units/%s/restore", encodedID)

	req := RestoreVersionRequest{
		Timestamp: versionTimestamp,
		LockID:    lockID,
	}

	resp, err := c.doJSON(ctx, "POST", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}

	return nil
}

// GetUnitStatus fetches dependency status for a unit
func (c *Client) GetUnitStatus(ctx context.Context, unitID string) (*UnitStatus, error) {
	encodedID := encodeUnitID(unitID)
	path := "/v1/units/" + encodedID + "/status"
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var st UnitStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &st, nil
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

// encodeUnitID encodes a unit ID for use in URLs by replacing slashes with double underscores
func encodeUnitID(id string) string {
	return strings.ReplaceAll(id, "/", "__")
}

// SetBearerToken sets the Authorization bearer token for subsequent requests.
func (c *Client) SetBearerToken(token string) { c.authToken = token }

// GetAuthToken returns the current authorization token
func (c *Client) GetAuthToken() string { return c.authToken }

// PostJSON makes a POST request with JSON payload
func (c *Client) PostJSON(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	return c.doJSON(ctx, "POST", path, payload)
}

// Get makes a GET request
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, "GET", path, nil)
}

// Delete makes a DELETE request
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, "DELETE", path, nil)
}
