package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	e2bRunsPath = "/api/v1/sandboxes/runs"
)

// e2bSandbox speaks to the Python/TypeScript sidecar that manages E2B sandboxes.
type e2bSandbox struct {
	cfg        E2BConfig
	httpClient *http.Client
}

// NewE2BSandbox constructs a sandbox implementation that delegates execution to an external sidecar.
func NewE2BSandbox(cfg E2BConfig) (Sandbox, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("E2B sandbox requires a base URL")
	}
	// Timeouts are configured via env vars in config.go with sensible defaults:
	// - PollInterval: 5s (OPENTACO_E2B_POLL_INTERVAL)
	// - PollTimeout: 30m (OPENTACO_E2B_POLL_TIMEOUT)
	// - HTTPTimeout: 60s (OPENTACO_E2B_HTTP_TIMEOUT)

	return &e2bSandbox{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}, nil
}

func (s *e2bSandbox) Name() string {
	return ProviderE2B
}

func (s *e2bSandbox) ExecutePlan(ctx context.Context, req *PlanRequest) (*PlanResult, error) {
	if req == nil {
		return nil, fmt.Errorf("plan request cannot be nil")
	}
	
	// Validate engine field is set
	if req.Engine == "" {
		return nil, fmt.Errorf("engine field is required but was empty")
	}
	if req.Engine != "terraform" && req.Engine != "tofu" {
		return nil, fmt.Errorf("invalid engine %q, must be 'terraform' or 'tofu'", req.Engine)
	}

	jobID, err := s.startRun(ctx, e2bRunRequest{
		Operation:              "plan",
		RunID:                  req.RunID,
		PlanID:                 req.PlanID,
		OrgID:                  req.OrgID,
		UnitID:                 req.UnitID,
		ConfigurationVersionID: req.ConfigurationVersionID,
		IsDestroy:              req.IsDestroy,
		TerraformVersion:       req.TerraformVersion,
		Engine:                 req.Engine,
		WorkingDirectory:       req.WorkingDirectory,
		ConfigArchive:          base64.StdEncoding.EncodeToString(req.ConfigArchive),
		State:                  encodeOptional(req.State),
		Metadata:               req.Metadata,
	})
	if err != nil {
		return nil, err
	}

	status, err := s.waitForCompletion(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if status.Result == nil {
		return nil, fmt.Errorf("sandbox run %s completed without a result payload", jobID)
	}

	var planJSON []byte
	if status.Result.PlanJSON != "" {
		decoded, err := base64.StdEncoding.DecodeString(status.Result.PlanJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode plan JSON from sandbox: %w", err)
		}
		planJSON = decoded
	}

	return &PlanResult{
		Logs:                 status.Logs,
		HasChanges:           boolValue(status.Result.HasChanges),
		ResourceAdditions:    intValue(status.Result.ResourceAdditions),
		ResourceChanges:      intValue(status.Result.ResourceChanges),
		ResourceDestructions: intValue(status.Result.ResourceDestructions),
		PlanJSON:             planJSON,
		RuntimeRunID:         status.ID,
	}, nil
}

func (s *e2bSandbox) ExecuteApply(ctx context.Context, req *ApplyRequest) (*ApplyResult, error) {
	if req == nil {
		return nil, fmt.Errorf("apply request cannot be nil")
	}
	
	// Validate engine field is set
	if req.Engine == "" {
		return nil, fmt.Errorf("engine field is required but was empty")
	}
	if req.Engine != "terraform" && req.Engine != "tofu" {
		return nil, fmt.Errorf("invalid engine %q, must be 'terraform' or 'tofu'", req.Engine)
	}

	jobID, err := s.startRun(ctx, e2bRunRequest{
		Operation:              "apply",
		RunID:                  req.RunID,
		PlanID:                 req.PlanID,
		OrgID:                  req.OrgID,
		UnitID:                 req.UnitID,
		ConfigurationVersionID: req.ConfigurationVersionID,
		IsDestroy:              req.IsDestroy,
		TerraformVersion:       req.TerraformVersion,
		Engine:                 req.Engine,
		WorkingDirectory:       req.WorkingDirectory,
		ConfigArchive:          base64.StdEncoding.EncodeToString(req.ConfigArchive),
		State:                  encodeOptional(req.State),
		Metadata:               req.Metadata,
	})
	if err != nil {
		return nil, err
	}

	status, err := s.waitForCompletion(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if status.Result == nil || status.Result.State == "" {
		return nil, fmt.Errorf("sandbox run %s completed without returning updated state", jobID)
	}

	stateBytes, err := base64.StdEncoding.DecodeString(status.Result.State)
	if err != nil {
		return nil, fmt.Errorf("failed to decode sandbox state payload: %w", err)
	}

	return &ApplyResult{
		Logs:         status.Logs,
		State:        stateBytes,
		RuntimeRunID: status.ID,
	}, nil
}

func (s *e2bSandbox) startRun(ctx context.Context, payload e2bRunRequest) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sandbox payload: %w", err)
	}

	// Retry logic for transient failures (network issues, sidecar temporarily unavailable)
	maxRetries := 3
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-2)) * time.Second
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint(e2bRunsPath), bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("failed to build sandbox request: %w", err)
		}
		s.decorateHeaders(req)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to start sandbox run (attempt %d/%d): %w", attempt, maxRetries, err)
			continue // Retry on network errors
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			// Server error - retry
			msg, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("sandbox returned %d (attempt %d/%d): %s", resp.StatusCode, attempt, maxRetries, strings.TrimSpace(string(msg)))
			continue
		}

		if resp.StatusCode >= 300 {
			// Client error - don't retry
			msg, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("sandbox returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
		}

		var startResp e2bRunStartResponse
		if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
			return "", fmt.Errorf("failed to decode sandbox start response: %w", err)
		}
		if startResp.ID == "" {
			return "", fmt.Errorf("sandbox did not return a run identifier")
		}
		return startResp.ID, nil
	}
	
	return "", fmt.Errorf("failed to start sandbox run after %d attempts: %w", maxRetries, lastErr)
}

func (s *e2bSandbox) waitForCompletion(ctx context.Context, runID string) (*e2bRunStatusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.PollTimeout)
	defer cancel()

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	var lastErr error

	for {
		status, err := s.fetchStatus(ctx, runID)
		if err == nil {
			switch strings.ToLower(status.Status) {
			case "succeeded", "completed", "done":
				return status, nil
			case "failed", "errored":
				if status.ErrorMessage != "" {
					return nil, fmt.Errorf("sandbox run %s failed: %s", runID, status.ErrorMessage)
				}
				return nil, fmt.Errorf("sandbox run %s failed without an error message", runID)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("timed out waiting for sandbox run %s (last error: %v): %w", runID, lastErr, ctx.Err())
			}
			return nil, fmt.Errorf("timed out waiting for sandbox run %s: %w", runID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (s *e2bSandbox) fetchStatus(ctx context.Context, runID string) (*e2bRunStatusResponse, error) {
	url := s.endpoint(fmt.Sprintf("%s/%s", e2bRunsPath, runID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build sandbox status request: %w", err)
	}
	s.decorateHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query sandbox status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox status returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var status e2bRunStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode sandbox status response: %w", err)
	}
	return &status, nil
}

func (s *e2bSandbox) decorateHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

func (s *e2bSandbox) endpoint(path string) string {
	if strings.HasPrefix(path, "/") {
		return s.cfg.BaseURL + path
	}
	return s.cfg.BaseURL + "/" + path
}

func encodeOptional(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func intValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

type e2bRunRequest struct {
	Operation              string            `json:"operation"`
	RunID                  string            `json:"run_id"`
	PlanID                 string            `json:"plan_id,omitempty"`
	OrgID                  string            `json:"org_id"`
	UnitID                 string            `json:"unit_id"`
	ConfigurationVersionID string            `json:"configuration_version_id"`
	IsDestroy              bool              `json:"is_destroy"`
	TerraformVersion       string            `json:"terraform_version,omitempty"`
	Engine                 string            `json:"engine,omitempty"`
	WorkingDirectory       string            `json:"working_directory,omitempty"`
	ConfigArchive          string            `json:"config_archive"`
	State                  string            `json:"state,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
}

type e2bRunStartResponse struct {
	ID string `json:"id"`
}

type e2bRunStatusResponse struct {
	ID           string                 `json:"id"`
	Operation    string                 `json:"operation"`
	Status       string                 `json:"status"`
	Logs         string                 `json:"logs"`
	Result       *e2bRunStatusResult    `json:"result,omitempty"`
	ErrorMessage string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type e2bRunStatusResult struct {
	HasChanges           *bool  `json:"has_changes,omitempty"`
	ResourceAdditions    *int   `json:"resource_additions,omitempty"`
	ResourceChanges      *int   `json:"resource_changes,omitempty"`
	ResourceDestructions *int   `json:"resource_destructions,omitempty"`
	PlanJSON             string `json:"plan_json,omitempty"`
	State                string `json:"state,omitempty"`
}
