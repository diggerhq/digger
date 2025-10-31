package config

import (
	"errors"
	"os"
)

// ConfigProvider is what the rest of the code depends on.
// Both Config and MockConfig will satisfy this.
type ConfigProvider interface {
	GetSecretKey() (string, error)
}

// Config is the real implementation that reads from env.
type Config struct{}

func (c *Config) GetSecretKey() (string, error) {
	secret := os.Getenv("OPENTACO_SECRET_KEY")
	if secret == "" {
		return "", errors.New("OPENTACO_SECRET_KEY environment variable not set")
	}
	return secret, nil
}

// MockConfig is a test double you can inject in tests.
type MockConfig struct {
	Secret string
	Err    error
}

func (m *MockConfig) GetSecretKey() (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Secret, nil
}

// active is the "current" config provider used by prod code.
// default is the real env-based config.
var active ConfigProvider = &Config{}

// GetConfig returns whichever provider is currently active.
func GetConfig() ConfigProvider {
	return active
}

// SetConfig lets tests (or special code) replace the active provider.
// You call this in tests to inject MockConfig.
// DO NOT call this in normal runtime code unless you really mean to override.
func SetConfig(p ConfigProvider) {
	active = p
}
