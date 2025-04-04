package config

import (
	"fmt"
	"os"
	"strconv"
)

// LimitByNumOfFilesChanged compat function until codebase is migrated to use the new config system
func LimitByNumOfFilesChanged() bool {
	// if this flag is set then it will fail if there are more projects impacted than the
	// number of files changed
	return BackendConfig.Features.LimitMaxProjectsToFilesChanged
}

// GetPort compat function until codebase is migrated to use the new config system
func GetPort() int {
	return BackendConfig.Server.Port
}

// SetEnvFromConfig sets environment variables based on the loaded configuration
// This is a compatibility layer to support code that still reads from os.Getenv
func SetEnvFromConfig(cfg *Config) {
	// Server config
	os.Setenv("DIGGER_PPROF_DEBUG_ENABLED", strconv.FormatBool(cfg.Server.PprofDebugEnabled))
	os.Setenv("DIGGER_ENABLE_INTERNAL_ENDPOINTS", strconv.FormatBool(cfg.Server.EnableInternalEndpoints))
	os.Setenv("DIGGER_ENABLE_API_ENDPOINTS", strconv.FormatBool(cfg.Server.EnableApiEndpoints))

	// Sentry config
	if cfg.Analytics.Sentry.Enabled {
		os.Setenv("SENTRY_DSN", cfg.Analytics.Sentry.DSN)
	}

	// GitHub config
	os.Setenv("GITHUB_WEBHOOK_SECRET", cfg.CI.GitHub.WebhookSecret)
	os.Setenv("GITHUB_APP_ID", strconv.Itoa(cfg.CI.GitHub.AppID))
	os.Setenv("GITHUB_PRIVATE_KEY_PATH", cfg.CI.GitHub.PrivateKeyPath)
	os.Setenv("GITHUB_APP_PRIVATE_KEY", cfg.CI.GitHub.AppPrivateKey)
	os.Setenv("GITHUB_APP_PRIVATE_KEY_BASE64", cfg.CI.GitHub.AppPrivateKeyBase64)
	os.Setenv("GITHUB_APP_CLIENT_ID", cfg.CI.GitHub.AppClientId)
	os.Setenv("GITHUB_APP_CLIENT_SECRET", cfg.CI.GitHub.AppClientSecret)

	// Auth config
	os.Setenv("INTERNAL_API_SECRET", cfg.Auth.InternalSecret)
	os.Setenv("FRONTEGG_CLIENT_ID", cfg.Auth.FronteggClientId)

	// Database config
	if cfg.Database.Type == "postgres" {
		os.Setenv("DATABASE_URL", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.Database.Postgres.User,
			cfg.Database.Postgres.Pass,
			cfg.Database.Postgres.Host,
			cfg.Database.Postgres.Port,
			cfg.Database.Postgres.Name,
			cfg.Database.Postgres.Ssl))
	}
}
