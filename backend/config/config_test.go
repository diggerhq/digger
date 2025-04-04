package config

import (
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		validate   func(*testing.T, *Config)
		wantErr    string
	}{
		{
			name:       "basic_config",
			configPath: "testdata/basic.yaml",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 4000, cfg.Server.Port)
				assert.Equal(t, "https://digger.example.com", cfg.Server.BaseURL)
				assert.Equal(t, "sqlite", cfg.Database.Type)
				assert.Equal(t, "/tmp/digger.db", cfg.Database.Sqlite.Path)
				assert.Equal(t, "github", cfg.CI.Provider)
				assert.Equal(t, "github.enterprise.com", cfg.CI.GitHub.Hostname)
				assert.Equal(t, "test-secret", cfg.CI.GitHub.WebhookSecret)
				assert.Equal(t, "github-token", cfg.CI.GitHub.Token)
			},
		},
		{
			name:       "gitlab_config",
			configPath: "testdata/gitlab.yaml",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 5000, cfg.Server.Port)
				assert.Equal(t, "https://digger-gitlab.example.com", cfg.Server.BaseURL)
				assert.Equal(t, "gitlab", cfg.CI.Provider)
				assert.Equal(t, "gitlab-token", cfg.CI.GitLab.AccessToken)
				assert.Equal(t, "https://gitlab.example.com", cfg.CI.GitLab.BaseURL)
			},
		},
		{
			name:       "ai_config",
			configPath: "testdata/ai_enabled.yaml",
			validate: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.AI.Enabled)

				assert.True(t, cfg.AI.Summary.Enabled)
				assert.Equal(t, "https://ai-summary.example.com", cfg.AI.Summary.Endpoint)
				assert.Equal(t, "summary-token", cfg.AI.Summary.ApiToken)
				assert.Equal(t, 1000, cfg.AI.Summary.MaxLength)
				assert.Equal(t, 60*time.Second, cfg.AI.Summary.Timeout)

				assert.True(t, cfg.AI.Generation.Enabled)
				assert.Equal(t, "https://ai-generation.example.com", cfg.AI.Generation.Endpoint)
				assert.Equal(t, "generation-token", cfg.AI.Generation.ApiToken)
				assert.Equal(t, 2000, cfg.AI.Generation.MaxTokens)
				assert.Equal(t, 0.5, cfg.AI.Generation.Temperature)
				assert.Equal(t, 120*time.Second, cfg.AI.Generation.Timeout)
			},
		},
		{
			name:       "invalid_log_level",
			configPath: "testdata/invalid_log_level.yaml",
			wantErr:    "invalid log level",
		},
		{
			name:       "invalid_log_format",
			configPath: "testdata/invalid_log_format.yaml",
			wantErr:    "invalid log format",
		},
		{
			name:       "invalid_database_type",
			configPath: "testdata/invalid_database.yaml",
			wantErr:    "database.type must be one of",
		},
		{
			name:       "missing_postgres_config",
			configPath: "testdata/missing_postgres.yaml",
			wantErr:    "database.postgres.host is required when",
		},
		{
			name:       "missing_jwt_auth",
			configPath: "testdata/missing_jwt.yaml",
			wantErr:    "either auth.jwt_secret or auth.jwt_public_key must be set when not using external authentication",
		},
		{
			name:       "missing_github_app_credentials",
			configPath: "testdata/github_app_missing.yaml",
			wantErr:    "when ci.github.app_id is set, you must provide",
		},
		{
			name:       "mutually_exclusive_github_keys",
			configPath: "testdata/github_app_conflict.yaml",
			wantErr:    "ci.github.app_private_key and ci.github.app_private_key_base64 are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the global config
			BackendConfig = nil

			cfg, err := LoadConfig(tt.configPath)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, cfg, BackendConfig)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		envVars    map[string]string
		validate   func(*testing.T, *Config)
	}{
		{
			name:       "basic_env_vars",
			configPath: "testdata/basic.yaml",
			envVars: map[string]string{
				"DIGGER_SERVER_PORT":            "5000",
				"DIGGER_SERVER_BASE_URL":        "https://digger-env.example.com",
				"DIGGER_DATABASE_TYPE":          "postgres",
				"DIGGER_DATABASE_POSTGRES_HOST": "postgres-env-host",
				"DIGGER_DATABASE_POSTGRES_PORT": "5433",
				"DIGGER_DATABASE_POSTGRES_NAME": "digger-env-db",
				"DIGGER_DATABASE_POSTGRES_USER": "postgres-env-user",
				"DIGGER_DATABASE_POSTGRES_PASS": "postgres-env-password",
				"DIGGER_CI_PROVIDER":            "github",
				"DIGGER_CI_GITHUB_HOSTNAME":     "github-env.enterprise.com",
				"DIGGER_AUTH_JWT_SECRET":        "env-jwt-secret",
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 5000, cfg.Server.Port)
				assert.Equal(t, "https://digger-env.example.com", cfg.Server.BaseURL)
				assert.Equal(t, "postgres", cfg.Database.Type)
				assert.Equal(t, "postgres-env-host", cfg.Database.Postgres.Host)
				assert.Equal(t, 5433, cfg.Database.Postgres.Port)
				assert.Equal(t, "digger-env-db", cfg.Database.Postgres.Name)
				assert.Equal(t, "postgres-env-user", cfg.Database.Postgres.User)
				assert.Equal(t, "postgres-env-password", cfg.Database.Postgres.Pass)
				assert.Equal(t, "github", cfg.CI.Provider)
				assert.Equal(t, "github-env.enterprise.com", cfg.CI.GitHub.Hostname)
				assert.Equal(t, "env-jwt-secret", cfg.Auth.JWTSecret)
			},
		},
		{
			name:       "feature_flags",
			configPath: "testdata/basic.yaml",
			envVars: map[string]string{
				"DIGGER_FEATURES_USER_SERVICE_ENABLED":                "false",
				"DIGGER_FEATURES_LIMIT_MAX_PROJECTS_TO_FILES_CHANGED": "true",
				"DIGGER_FEATURES_INTERNAL_USERS_ENABLED":              "true",
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.False(t, cfg.Features.UserServiceEnabled)
				assert.True(t, cfg.Features.LimitMaxProjectsToFilesChanged)
				assert.True(t, cfg.Features.InternalUsersEnabled)
			},
		},
		{
			name:       "ai_settings",
			configPath: "testdata/basic.yaml",
			envVars: map[string]string{
				"DIGGER_AI_ENABLED":                "true",
				"DIGGER_AI_SUMMARY_ENABLED":        "true",
				"DIGGER_AI_SUMMARY_ENDPOINT":       "https://ai-summary-env.example.com",
				"DIGGER_AI_SUMMARY_API_TOKEN":      "env-summary-token",
				"DIGGER_AI_GENERATION_ENABLED":     "true",
				"DIGGER_AI_GENERATION_ENDPOINT":    "https://ai-generation-env.example.com",
				"DIGGER_AI_GENERATION_API_TOKEN":   "env-generation-token",
				"DIGGER_AI_GENERATION_TEMPERATURE": "0.3",
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.AI.Enabled)
				assert.True(t, cfg.AI.Summary.Enabled)
				assert.Equal(t, "https://ai-summary-env.example.com", cfg.AI.Summary.Endpoint)
				assert.Equal(t, "env-summary-token", cfg.AI.Summary.ApiToken)
				assert.True(t, cfg.AI.Generation.Enabled)
				assert.Equal(t, "https://ai-generation-env.example.com", cfg.AI.Generation.Endpoint)
				assert.Equal(t, "env-generation-token", cfg.AI.Generation.ApiToken)
				assert.Equal(t, 0.3, cfg.AI.Generation.Temperature)
			},
		},
		{
			name:       "log_settings",
			configPath: "testdata/basic.yaml",
			envVars: map[string]string{
				"DIGGER_LOG_LEVEL":  "debug",
				"DIGGER_LOG_FORMAT": "json",
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "json", cfg.Log.Format)
				assert.Equal(t, int(cfg.Log.Level), int(slog.LevelDebug))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the global config
			BackendConfig = nil

			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := LoadConfig(tt.configPath)
			require.NoError(t, err)
			assert.NotNil(t, cfg)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}

			// Verify global config is set
			assert.Equal(t, cfg, BackendConfig)
		})
	}
}
