package sandbox

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// ProviderE2B enables the E2B-powered sandbox sidecar.
	ProviderE2B = "e2b"
)

// E2BConfig contains the settings needed to talk to the sidecar service that speaks to E2B.
type E2BConfig struct {
	BaseURL      string
	PollInterval time.Duration
	PollTimeout  time.Duration
	HTTPTimeout  time.Duration
}

// NewFromEnv returns the sandbox provider configured via environment variables.
// Returns (nil, nil) when no sandbox provider is configured.
func NewFromEnv() (Sandbox, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("OPENTACO_SANDBOX_PROVIDER")))
	if provider == "" || provider == "none" || provider == "disabled" {
		return nil, nil
	}

	switch provider {
	case ProviderE2B:
		cfg, err := loadE2BConfigFromEnv()
		if err != nil {
			return nil, err
		}
		return NewE2BSandbox(cfg)
	default:
		return nil, fmt.Errorf("unsupported sandbox provider %q", provider)
	}
}

func loadE2BConfigFromEnv() (E2BConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv("OPENTACO_E2B_SIDECAR_URL"))
	if baseURL == "" {
		return E2BConfig{}, fmt.Errorf("OPENTACO_E2B_SIDECAR_URL is required when using the E2B sandbox provider")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	pollInterval, err := parseDurationWithDefault(os.Getenv("OPENTACO_E2B_POLL_INTERVAL"), 5*time.Second)
	if err != nil {
		return E2BConfig{}, fmt.Errorf("invalid OPENTACO_E2B_POLL_INTERVAL: %w", err)
	}

	pollTimeout, err := parseDurationWithDefault(os.Getenv("OPENTACO_E2B_POLL_TIMEOUT"), 30*time.Minute)
	if err != nil {
		return E2BConfig{}, fmt.Errorf("invalid OPENTACO_E2B_POLL_TIMEOUT: %w", err)
	}

	httpTimeout, err := parseDurationWithDefault(os.Getenv("OPENTACO_E2B_HTTP_TIMEOUT"), 60*time.Second)
	if err != nil {
		return E2BConfig{}, fmt.Errorf("invalid OPENTACO_E2B_HTTP_TIMEOUT: %w", err)
	}

	return E2BConfig{
		BaseURL:      baseURL,
		PollInterval: pollInterval,
		PollTimeout:  pollTimeout,
		HTTPTimeout:  httpTimeout,
	}, nil
}

func parseDurationWithDefault(value string, def time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return def, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
