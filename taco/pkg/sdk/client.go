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
    ID           string    `json:"id"`
    Size         int64     `json:"size"`
    Updated      time.Time `json:"updated"`
    Locked       bool      `json:"locked"`
    LockInfo     *LockInfo `json:"lock,omitempty"`
    Tags         []string  `json:"tags,omitempty"`
    Description  string    `json:"description,omitempty"`
    Organization string    `json:"organization,omitempty"`
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
    ID string `json:"id"`
}

// CreateUnitResponse represents the response from creating a unit
type CreateUnitResponse struct {
    ID      string    `json:"id"`
    Created time.Time `json:"created"`
}

// ListUnitsResponse represents the response from listing units
type ListUnitsResponse struct {
    Units []*UnitMetadata `json:"units"`
    Count  int              `json:"count"`
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
    UnitID string         `json:"unit_id"`
    Status  string         `json:"status"`
    Incoming []IncomingEdge `json:"incoming"`
    Summary Summary        `json:"summary"`
}

type IncomingEdge struct {
    EdgeID      string `json:"edge_id,omitempty"`
    FromUnitID  string `json:"from_unit_id"`
    FromOutput  string `json:"from_output"`
    Status      string `json:"status"`
    InDigest    string `json:"in_digest,omitempty"`
    OutDigest   string `json:"out_digest,omitempty"`
    LastInAt    string `json:"last_in_at,omitempty"`
    LastOutAt   string `json:"last_out_at,omitempty"`
}

type Summary struct {
    IncomingOK      int `json:"incoming_ok"`
    IncomingPending int `json:"incoming_pending"`
    IncomingUnknown int `json:"incoming_unknown"`
}

// CreateUnit creates a new unit
func (c *Client) CreateUnit(ctx context.Context, unitID string) (*CreateUnitResponse, error) {
    req := CreateUnitRequest{ID: unitID}
    
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

//
// Tag Management Methods (using TFE API compatibility layer)
//

// CreateUnitWithMetadata creates a unit with tags and metadata using the TFE workspace API
func (c *Client) CreateUnitWithMetadata(ctx context.Context, unitID string, tags []string, description, organization string) (*UnitMetadata, error) {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "workspaces",
			"attributes": map[string]interface{}{
				"name":        unitID,
				"description": description,
				"tag-names":   tags,
			},
		},
	}

	resp, err := c.doJSON(ctx, "POST", fmt.Sprintf("/tfe/api/v2/organizations/%s/workspaces", organization), payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create unit with metadata: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Attributes struct {
				Name        string    `json:"name"`
				Description string    `json:"description"`
				TagNames    []string  `json:"tag-names"`
				CreatedAt   time.Time `json:"created-at"`
				UpdatedAt   time.Time `json:"updated-at"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &UnitMetadata{
		ID:           result.Data.Attributes.Name,
		Size:         0,
		Updated:      result.Data.Attributes.UpdatedAt,
		Locked:       false,
		Tags:         result.Data.Attributes.TagNames,
		Description:  result.Data.Attributes.Description,
		Organization: organization,
	}, nil
}

// ListUnitsByTags lists units filtered by tags and organization using TFE workspace API
func (c *Client) ListUnitsByTags(ctx context.Context, organization string, tags []string) ([]*UnitMetadata, error) {
	path := fmt.Sprintf("/tfe/api/v2/organizations/%s/workspaces", organization)
	if len(tags) > 0 {
		tagQuery := strings.Join(tags, ",")
		path += "?search[tags]=" + url.QueryEscape(tagQuery)
	}

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list units by tags: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Attributes struct {
				Name        string    `json:"name"`
				Description string    `json:"description"`
				TagNames    []string  `json:"tag-names"`
				CreatedAt   time.Time `json:"created-at"`
				UpdatedAt   time.Time `json:"updated-at"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var units []*UnitMetadata
	for _, item := range result.Data {
		units = append(units, &UnitMetadata{
			ID:           item.Attributes.Name,
			Size:         0, // TFE API doesn't expose size
			Updated:      item.Attributes.UpdatedAt,
			Locked:       false,
			Tags:         item.Attributes.TagNames,
			Description:  item.Attributes.Description,
			Organization: organization,
		})
	}

	return units, nil
}

// TagManagement represents tag metadata
type TagManagement struct {
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UnitCount   int       `json:"unit_count"`
}

// CreateTag creates a new tag via REST API
func (c *Client) CreateTag(ctx context.Context, name string) (*TagManagement, error) {
	payload := map[string]string{
		"name": name,
	}

	resp, err := c.doJSON(ctx, "POST", "/v1/tags", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("tag already exists")
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create tag: HTTP %d", resp.StatusCode)
	}

	var result TagManagement
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListTags lists all available tags via REST API
func (c *Client) ListTags(ctx context.Context) ([]*TagManagement, error) {
	resp, err := c.do(ctx, "GET", "/v1/tags", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list tags: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Tags []TagManagement `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var tags []*TagManagement
	for i := range result.Tags {
		tags = append(tags, &result.Tags[i])
	}

	return tags, nil
}

// GetTag gets information about a specific tag via REST API
func (c *Client) GetTag(ctx context.Context, tagName string) (*TagManagement, error) {
	resp, err := c.do(ctx, "GET", fmt.Sprintf("/v1/tags/%s", tagName), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tag not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tag: HTTP %d", resp.StatusCode)
	}

	var result TagManagement
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteTag deletes a tag via REST API  
func (c *Client) DeleteTag(ctx context.Context, tagName string) error {
	resp, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/tags/%s", tagName), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("tag not found")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete tag: HTTP %d", resp.StatusCode)
	}

	return nil
}

// GetUnitsByTag gets all units that have a specific tag via REST API
func (c *Client) GetUnitsByTag(ctx context.Context, tagName string) ([]*UnitMetadata, error) {
	resp, err := c.do(ctx, "GET", fmt.Sprintf("/v1/tags/%s/units", tagName), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get units by tag: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Units []struct {
			ID           string   `json:"id"`
			Size         int64    `json:"size"`
			Updated      string   `json:"updated"`
			Locked       bool     `json:"locked"`
			Tags         []string `json:"tags"`
			Description  string   `json:"description"`
			Organization string   `json:"organization"`
		} `json:"units"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var units []*UnitMetadata
	for _, unit := range result.Units {
		updated, _ := time.Parse("2006-01-02T15:04:05Z", unit.Updated)
		units = append(units, &UnitMetadata{
			ID:           unit.ID,
			Size:         unit.Size,
			Updated:      updated,
			Locked:       unit.Locked,
			Tags:         unit.Tags,
			Description:  unit.Description,
			Organization: unit.Organization,
		})
	}

	return units, nil
}

// AddTagToUnit adds a tag to a unit via REST API
func (c *Client) AddTagToUnit(ctx context.Context, unitID, tagName, organization string) error {
	payload := map[string]string{
		"tag_name": tagName,
	}

	resp, err := c.doJSON(ctx, "POST", fmt.Sprintf("/v1/units/%s/tags", unitID), payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("unit or tag not found")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add tag to unit: HTTP %d", resp.StatusCode)
	}

	return nil
}

// RemoveTagFromUnit removes a tag from a unit via REST API
func (c *Client) RemoveTagFromUnit(ctx context.Context, unitID, tagName, organization string) error {
	resp, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/units/%s/tags/%s", unitID, tagName), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("unit or tag not found")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove tag from unit: HTTP %d", resp.StatusCode)
	}

	return nil
}

// GetTagsForUnit returns all tags for a specific unit via REST API
func (c *Client) GetTagsForUnit(ctx context.Context, unitID, organization string) ([]string, error) {
	resp, err := c.do(ctx, "GET", fmt.Sprintf("/v1/units/%s/tags", unitID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("unit not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get unit tags: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Tags, nil
}
