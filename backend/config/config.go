package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

var (
	ErrMissingRequiredConfig = errors.New("missing required configuration")

	// BackendConfig stores the global application configuration (we will remove this once the code is fully migrated to this config)
	BackendConfig *Config

	// DiggerConfig offers backwards compatiblity until codebase is migrated (we will remove this once the code is fully migrated to this config)
	DiggerConfig *Config

	// validator instance
	validate = validator.New()
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig    `validate:"required"`
	Database  DatabaseConfig  `validate:"required"`
	Auth      AuthConfig      `validate:"required"`
	CI        CIConfig        `validate:"required"`
	Features  FeatureConfig   `validate:"required"`
	Analytics AnalyticsConfig `validate:"required"`
	Security  SecurityConfig  `validate:"required"`
	AI        AIConfig        `validate:"required"`
	Log       LogConfig       `validate:"required"`
}

// ServerConfig contains the server-related configuration
type ServerConfig struct {
	BackendHostname         string `validate:"omitempty"`
	Port                    int    `validate:"required,gt=0,lt=65536"`
	BaseURL                 string `validate:"required,url"`
	BuildDate               string `validate:"omitempty"`
	DeployedAt              string `validate:"omitempty"`
	MaxConcurrencyPerBatch  int    `validate:"omitempty,gte=0"`
	EnableInternalEndpoints bool
	EnableApiEndpoints      bool
	PprofDebugEnabled       bool
	WebhookTimeoutSeconds   int `validate:"required,gt=0"`
	RepoAllowList           string
	Pprof                   PprofConfig `validate:"required"`
}

// PprofConfig contains configuration for pprof profiling
type PprofConfig struct {
	Enabled         bool
	PeriodicEnabled bool
	Dir             string `validate:"required_if=PeriodicEnabled true"`
	IntervalMinutes int    `validate:"required_if=PeriodicEnabled true,gt=0"`
	KeepProfiles    int    `validate:"required_if=PeriodicEnabled true,gt=0"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type  string `validate:"required,oneof=sqlite postgres"`
	Debug bool

	Gorm GormConfig `validate:"required"`

	Sqlite   SqliteConfig   `validate:"required_if=Type sqlite"`
	Postgres PostgresConfig `validate:"required_if=Type postgres"`
}

// GormConfig contains GORM-specific configuration
type GormConfig struct {
	Debug                 bool
	SlowThreshold         time.Duration `validate:"omitempty,gt=0"`
	SkipErrRecordNotFound bool
	ParameterizedQueries  bool
	PrepareStmt           bool
}

// SqliteConfig contains SQLite-specific configuration
type SqliteConfig struct {
	Path              string `validate:"required"`
	WriteAheadLog     bool
	WALAutoCheckPoint int `validate:"omitempty,gte=0"`
}

// PostgresConfig contains PostgreSQL-specific configuration
type PostgresConfig struct {
	Host                string `validate:"required"`
	Port                int    `validate:"required,gt=0,lt=65536"`
	Name                string `validate:"required"`
	User                string `validate:"required"`
	Pass                string
	Ssl                 string `validate:"omitempty,oneof=disable require verify-ca verify-full"`
	MaxOpenConnections  int    `validate:"omitempty,gt=0"`
	MaxIdleConnections  int    `validate:"omitempty,gte=0"`
	ConnMaxIdleTimeSecs int    `validate:"omitempty,gte=0"`
}

// AuthConfig contains authentication-related configuration
type AuthConfig struct {
	JWTSecret         string
	JWTPublicKey      string
	BasicAuthEnabled  bool
	BasicAuthUsername string `validate:"required_if=BasicAuthEnabled true"`
	BasicAuthPassword string `validate:"required_if=BasicAuthEnabled true"`
	BearerAuthToken   string
	InternalSecret    string
	AuthHost          string
	AuthSecret        string
	FronteggClientId  string
}

// CIConfig contains CI provider configuration
type CIConfig struct {
	Provider string       `validate:"required,oneof=github gitlab"`
	GitHub   GitHubConfig `validate:"required_if=Provider github"`
	GitLab   GitLabConfig `validate:"required_if=Provider gitlab"`
}

// GitHubConfig contains GitHub-related configuration
type GitHubConfig struct {
	WebhookSecret       string
	AppID               int
	PrivateKeyPath      string
	WebhookTimeoutSecs  int `validate:"omitempty,gt=0"`
	Token               string
	Hostname            string `validate:"required,hostname"`
	AppPrivateKey       string
	AppPrivateKeyBase64 string
	AppClientId         string
	AppClientSecret     string
}

// GitLabConfig contains GitLab-related configuration
type GitLabConfig struct {
	AccessToken string `validate:"required"`
	BaseURL     string `validate:"required,url"`
}

// FeatureConfig contains feature flag settings
type FeatureConfig struct {
	UserServiceEnabled             bool
	LimitMaxProjectsToFilesChanged bool
	InternalUsersEnabled           bool
}

// AnalyticsConfig contains analytics-related configuration
type AnalyticsConfig struct {
	Segment SegmentConfig `validate:"required"`
	Sentry  SentryConfig  `validate:"required"`
}

// SegmentConfig contains Segment-specific configuration
type SegmentConfig struct {
	Enabled bool
	ApiKey  string `validate:"required_if=Enabled true"`
}

// SentryConfig contains Sentry-specific configuration
type SentryConfig struct {
	Enabled          bool
	DSN              string `validate:"required_if=Enabled true,omitempty,url"`
	Debug            bool
	EnableTracing    bool
	TracesSampleRate float64 `validate:"omitempty,gte=0,lte=1"`
	Environment      string
	Release          string
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	EncryptionSecret string
}

// AIConfig contains all AI/ML-related configuration
type AIConfig struct {
	Enabled    bool
	Summary    AISummaryConfig    `validate:"required"`
	Generation AIGenerationConfig `validate:"required"`
}

// AISummaryConfig contains configuration for AI summary feature
type AISummaryConfig struct {
	Enabled   bool
	Endpoint  string        `validate:"required_if=Enabled true,omitempty,url"`
	ApiToken  string        `validate:"required_if=Enabled true"`
	MaxLength int           `validate:"omitempty,gt=0"`
	Timeout   time.Duration `validate:"omitempty,gt=0"`
}

// AIGenerationConfig contains configuration for AI generation feature
type AIGenerationConfig struct {
	Enabled     bool
	Endpoint    string        `validate:"required_if=Enabled true,omitempty,url"`
	ApiToken    string        `validate:"required_if=Enabled true"`
	MaxTokens   int           `validate:"omitempty,gt=0"`
	Temperature float64       `validate:"omitempty,gte=0,lte=1"`
	Timeout     time.Duration `validate:"omitempty,gt=0"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Format string `validate:"required,oneof=json text"`
	Level  slog.Level
}

// LoadConfig prepares and loads the Digger configuration
// This sets default values, reads configuration files, and handles environment variables
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Setup viper to handle environment variables
	v.SetEnvPrefix("DIGGER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Set default values
	setDefaultValues(v)

	// Look for config files in the standard locations
	v.SetConfigName("config")
	v.AddConfigPath("/etc/digger-backend/")
	v.AddConfigPath("$HOME/.digger-backend")
	v.AddConfigPath(".")

	// Read config file if specified
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		slog.Debug("Config file loaded", "config_file", v.ConfigFileUsed())
	}

	// Configure logging
	logConfig, err := configureLogging(v)
	if err != nil {
		return nil, err
	}

	// Build the config structure
	cfg := buildConfigFromViper(v, logConfig)

	// Validate the configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	// Perform server-specific validation
	if err := validateServerConfig(cfg); err != nil {
		return nil, err
	}

	// Set the global configuration
	// TODO: global variables are evil! Remove this once the codebase has fully migrated to use the newer config system
	BackendConfig = cfg
	DiggerConfig = cfg

	return cfg, nil
}

// setDefaultValues sets the default values for configuration options
func setDefaultValues(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 3000)
	v.SetDefault("server.build_date", "unknown")
	v.SetDefault("server.deployed_at", time.Now().UTC().Format(time.RFC3339))
	v.SetDefault("server.max_concurrency_per_batch", 0)
	v.SetDefault("server.base_url", "http://localhost:3000")
	v.SetDefault("server.webhook_timeout_seconds", 10)
	v.SetDefault("server.enable_internal_endpoints", false)
	v.SetDefault("server.enable_api_endpoints", true)
	v.SetDefault("server.pprof_debug_enabled", false)
	v.SetDefault("server.repo_allow_list", "")
	v.SetDefault("server.backend_hostname", "")

	// Pprof defaults
	v.SetDefault("server.pprof.enabled", false)
	v.SetDefault("server.pprof.periodic_enabled", false)
	v.SetDefault("server.pprof.dir", "/tmp/profiles")
	v.SetDefault("server.pprof.interval_minutes", 60)
	v.SetDefault("server.pprof.keep_profiles", 168)

	// Database defaults
	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.debug", false)

	// Gorm defaults
	v.SetDefault("database.gorm.debug", false)
	v.SetDefault("database.gorm.slow_threshold", 200*time.Millisecond)
	v.SetDefault("database.gorm.skip_err_record_not_found", false)
	v.SetDefault("database.gorm.parameterized_queries", false)
	v.SetDefault("database.gorm.prepare_stmt", false)

	// SQLite defaults
	v.SetDefault("database.sqlite.path", "digger.db")
	v.SetDefault("database.sqlite.write_ahead_log", true)
	v.SetDefault("database.sqlite.wal_auto_check_point", 1000)

	// PostgreSQL defaults
	v.SetDefault("database.postgres.host", "localhost")
	v.SetDefault("database.postgres.port", 5432)
	v.SetDefault("database.postgres.name", "digger")
	v.SetDefault("database.postgres.user", "postgres")
	v.SetDefault("database.postgres.pass", "")
	v.SetDefault("database.postgres.ssl", "disable")
	v.SetDefault("database.postgres.max_open_connections", 100)
	v.SetDefault("database.postgres.max_idle_connections", 10)
	v.SetDefault("database.postgres.conn_max_idle_time_secs", 300)

	// Auth defaults
	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.jwt_public_key", "")
	v.SetDefault("auth.basic_auth_enabled", false)
	v.SetDefault("auth.basic_auth_username", "")
	v.SetDefault("auth.basic_auth_password", "")
	v.SetDefault("auth.bearer_auth_token", "")
	v.SetDefault("auth.internal_secret", "")
	v.SetDefault("auth.auth_host", "")
	v.SetDefault("auth.auth_secret", "")
	v.SetDefault("auth.frontegg_client_id", "")

	// CI Provider defaults
	v.SetDefault("ci.provider", "github")

	// GitHub defaults
	v.SetDefault("ci.github.webhook_secret", "")
	v.SetDefault("ci.github.app_id", 0)
	v.SetDefault("ci.github.token", "")
	v.SetDefault("ci.github.webhook_timeout_secs", 10)
	v.SetDefault("ci.github.hostname", "github.com")
	v.SetDefault("ci.github.app_private_key", "")
	v.SetDefault("ci.github.app_private_key_base64", "")
	v.SetDefault("ci.github.app_client_id", "")
	v.SetDefault("ci.github.app_client_secret", "")

	// GitLab defaults
	v.SetDefault("ci.gitlab.access_token", "")
	v.SetDefault("ci.gitlab.base_url", "https://gitlab.com")

	// Features defaults
	v.SetDefault("features.user_service_enabled", true)
	v.SetDefault("features.limit_max_projects_to_files_changed", false)
	v.SetDefault("features.internal_users_enabled", false)

	// Analytics defaults - Segment
	v.SetDefault("analytics.segment.enabled", false)
	v.SetDefault("analytics.segment.api_key", "")

	// Analytics defaults - Sentry
	v.SetDefault("analytics.sentry.enabled", false)
	v.SetDefault("analytics.sentry.dsn", "")
	v.SetDefault("analytics.sentry.debug", false)
	v.SetDefault("analytics.sentry.enable_tracing", true)
	v.SetDefault("analytics.sentry.traces_sample_rate", 0.1)
	v.SetDefault("analytics.sentry.environment", "development")
	v.SetDefault("analytics.sentry.release", "")

	// Security defaults
	v.SetDefault("security.encryption_secret", "")

	// AI global defaults
	v.SetDefault("ai.enabled", false)

	// AI Summary defaults
	v.SetDefault("ai.summary.enabled", false)
	v.SetDefault("ai.summary.endpoint", "")
	v.SetDefault("ai.summary.api_token", "")
	v.SetDefault("ai.summary.max_length", 500)
	v.SetDefault("ai.summary.timeout", 30*time.Second)

	// AI Generation defaults
	v.SetDefault("ai.generation.enabled", false)
	v.SetDefault("ai.generation.endpoint", "")
	v.SetDefault("ai.generation.api_token", "")
	v.SetDefault("ai.generation.max_tokens", 1000)
	v.SetDefault("ai.generation.temperature", 0)
	v.SetDefault("ai.generation.timeout", 60*time.Second)

	// Logging defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
}

// buildConfigFromViper creates a Config struct from viper settings
func buildConfigFromViper(v *viper.Viper, logConfig LogConfig) *Config {
	return &Config{
		Server: ServerConfig{
			BackendHostname:         v.GetString("server.backend_hostname"),
			Port:                    v.GetInt("server.port"),
			BaseURL:                 v.GetString("server.base_url"),
			BuildDate:               v.GetString("server.build_date"),
			DeployedAt:              v.GetString("server.deployed_at"),
			MaxConcurrencyPerBatch:  v.GetInt("server.max_concurrency_per_batch"),
			EnableInternalEndpoints: v.GetBool("server.enable_internal_endpoints"),
			EnableApiEndpoints:      v.GetBool("server.enable_api_endpoints"),
			PprofDebugEnabled:       v.GetBool("server.pprof_debug_enabled"),
			WebhookTimeoutSeconds:   v.GetInt("server.webhook_timeout_seconds"),
			RepoAllowList:           v.GetString("server.repo_allow_list"),
			Pprof: PprofConfig{
				Enabled:         v.GetBool("server.pprof.enabled"),
				PeriodicEnabled: v.GetBool("server.pprof.periodic_enabled"),
				Dir:             v.GetString("server.pprof.dir"),
				IntervalMinutes: v.GetInt("server.pprof.interval_minutes"),
				KeepProfiles:    v.GetInt("server.pprof.keep_profiles"),
			},
		},

		Database: DatabaseConfig{
			Type:  v.GetString("database.type"),
			Debug: v.GetBool("database.debug"),

			Gorm: GormConfig{
				Debug:                 v.GetBool("database.gorm.debug"),
				SlowThreshold:         v.GetDuration("database.gorm.slow_threshold"),
				SkipErrRecordNotFound: v.GetBool("database.gorm.skip_err_record_not_found"),
				ParameterizedQueries:  v.GetBool("database.gorm.parameterized_queries"),
				PrepareStmt:           v.GetBool("database.gorm.prepare_stmt"),
			},

			Sqlite: SqliteConfig{
				Path:              v.GetString("database.sqlite.path"),
				WriteAheadLog:     v.GetBool("database.sqlite.write_ahead_log"),
				WALAutoCheckPoint: v.GetInt("database.sqlite.wal_auto_check_point"),
			},

			Postgres: PostgresConfig{
				Host:                v.GetString("database.postgres.host"),
				Port:                v.GetInt("database.postgres.port"),
				Name:                v.GetString("database.postgres.name"),
				User:                v.GetString("database.postgres.user"),
				Pass:                v.GetString("database.postgres.pass"),
				Ssl:                 v.GetString("database.postgres.ssl"),
				MaxOpenConnections:  v.GetInt("database.postgres.max_open_connections"),
				MaxIdleConnections:  v.GetInt("database.postgres.max_idle_connections"),
				ConnMaxIdleTimeSecs: v.GetInt("database.postgres.conn_max_idle_time_secs"),
			},
		},

		Auth: AuthConfig{
			JWTSecret:         v.GetString("auth.jwt_secret"),
			JWTPublicKey:      v.GetString("auth.jwt_public_key"),
			BasicAuthEnabled:  v.GetBool("auth.basic_auth_enabled"),
			BasicAuthUsername: v.GetString("auth.basic_auth_username"),
			BasicAuthPassword: v.GetString("auth.basic_auth_password"),
			BearerAuthToken:   v.GetString("auth.bearer_auth_token"),
			InternalSecret:    v.GetString("auth.internal_secret"),
			AuthHost:          v.GetString("auth.auth_host"),
			AuthSecret:        v.GetString("auth.auth_secret"),
			FronteggClientId:  v.GetString("auth.frontegg_client_id"),
		},

		CI: CIConfig{
			Provider: v.GetString("ci.provider"),

			GitHub: GitHubConfig{
				WebhookSecret:       v.GetString("ci.github.webhook_secret"),
				AppID:               v.GetInt("ci.github.app_id"),
				PrivateKeyPath:      absolutePathFromConfigPath(v.GetString("ci.github.private_key_path")),
				WebhookTimeoutSecs:  v.GetInt("ci.github.webhook_timeout_secs"),
				Token:               v.GetString("ci.github.token"),
				Hostname:            v.GetString("ci.github.hostname"),
				AppPrivateKey:       v.GetString("ci.github.app_private_key"),
				AppPrivateKeyBase64: v.GetString("ci.github.app_private_key_base64"),
				AppClientId:         v.GetString("ci.github.app_client_id"),
				AppClientSecret:     v.GetString("ci.github.app_client_secret"),
			},

			GitLab: GitLabConfig{
				AccessToken: v.GetString("ci.gitlab.access_token"),
				BaseURL:     v.GetString("ci.gitlab.base_url"),
			},
		},

		Features: FeatureConfig{
			UserServiceEnabled:             v.GetBool("features.user_service_enabled"),
			LimitMaxProjectsToFilesChanged: v.GetBool("features.limit_max_projects_to_files_changed"),
			InternalUsersEnabled:           v.GetBool("features.internal_users_enabled"),
		},

		Analytics: AnalyticsConfig{
			Segment: SegmentConfig{
				Enabled: v.GetBool("analytics.segment.enabled"),
				ApiKey:  v.GetString("analytics.segment.api_key"),
			},

			Sentry: SentryConfig{
				Enabled:          v.GetBool("analytics.sentry.enabled"),
				DSN:              v.GetString("analytics.sentry.dsn"),
				Debug:            v.GetBool("analytics.sentry.debug"),
				EnableTracing:    v.GetBool("analytics.sentry.enable_tracing"),
				TracesSampleRate: v.GetFloat64("analytics.sentry.traces_sample_rate"),
				Environment:      v.GetString("analytics.sentry.environment"),
				Release:          v.GetString("analytics.sentry.release"),
			},
		},

		Security: SecurityConfig{
			EncryptionSecret: v.GetString("security.encryption_secret"),
		},

		AI: AIConfig{
			Enabled: v.GetBool("ai.enabled"),

			Summary: AISummaryConfig{
				Enabled:   v.GetBool("ai.summary.enabled"),
				Endpoint:  v.GetString("ai.summary.endpoint"),
				ApiToken:  v.GetString("ai.summary.api_token"),
				MaxLength: v.GetInt("ai.summary.max_length"),
				Timeout:   v.GetDuration("ai.summary.timeout"),
			},

			Generation: AIGenerationConfig{
				Enabled:     v.GetBool("ai.generation.enabled"),
				Endpoint:    v.GetString("ai.generation.endpoint"),
				ApiToken:    v.GetString("ai.generation.api_token"),
				MaxTokens:   v.GetInt("ai.generation.max_tokens"),
				Temperature: v.GetFloat64("ai.generation.temperature"),
				Timeout:     v.GetDuration("ai.generation.timeout"),
			},
		},

		Log: logConfig,
	}
}

// configureLogging creates a LogConfig struct from viper settings
func configureLogging(v *viper.Viper) (LogConfig, error) {
	logLevelStr := v.GetString("log.level")
	var logLevel slog.Level

	switch strings.ToLower(logLevelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return LogConfig{}, fmt.Errorf("invalid log level %q: must be one of debug, info, warn, error", logLevelStr)
	}

	logFormatOpt := v.GetString("log.format")
	var logFormat string
	switch logFormatOpt {
	case "json":
		logFormat = "json"
		opts := &slog.HandlerOptions{Level: logLevel}
		handler := slog.NewJSONHandler(os.Stderr, opts)
		logger := slog.New(handler)
		slog.SetDefault(logger)
	case "text", "":
		logFormat = "text"
		opts := &slog.HandlerOptions{Level: logLevel}
		handler := slog.NewTextHandler(os.Stderr, opts)
		logger := slog.New(handler)
		slog.SetDefault(logger)
	default:
		return LogConfig{}, fmt.Errorf("invalid log format %q: must be 'json' or 'text'", logFormatOpt)
	}

	return LogConfig{
		Format: logFormat,
		Level:  logLevel,
	}, nil
}

// absolutePathFromConfigPath makes a path absolute if it isn't already
func absolutePathFromConfigPath(path string) string {
	if path == "" {
		return ""
	}

	// Handle home directory expansion
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// If path is already absolute, return it
	if filepath.IsAbs(path) {
		return path
	}

	// Otherwise, make it absolute relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		slog.Warn("Failed to get absolute path", "path", path, "error", err)
		return path
	}

	return absPath
}

// validateConfig performs validation using the validator package
func validateConfig(cfg *Config) error {
	err := validate.Struct(cfg)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errorMessages []string
			for _, e := range validationErrors {
				errorMessages = append(errorMessages, formatValidationError(e))
			}
			return fmt.Errorf("configuration validation failed: %s", strings.Join(errorMessages, "; "))
		}
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	return nil
}

// validateServerConfig performs server-specific configuration validation
func validateServerConfig(cfg *Config) error {
	// First do basic validation
	if err := validateConfig(cfg); err != nil {
		return err
	}

	// Additional server-specific validation that can't be done with struct tags
	var validationErrors []string

	// Check for required JWT auth if not using external auth
	if cfg.Auth.AuthHost == "" && cfg.Auth.JWTSecret == "" && cfg.Auth.JWTPublicKey == "" {
		validationErrors = append(validationErrors, "either auth.jwt_secret or auth.jwt_public_key must be set when not using external authentication")
	}

	// Check GitHub integration if using GitHub
	if cfg.CI.Provider == "github" || cfg.CI.Provider == "both" {
		if cfg.CI.GitHub.AppID != 0 {
			if cfg.CI.GitHub.AppPrivateKey == "" && cfg.CI.GitHub.AppPrivateKeyBase64 == "" && cfg.CI.GitHub.PrivateKeyPath == "" {
				validationErrors = append(validationErrors, "when ci.github.app_id is set, you must provide ci.github.app_private_key, ci.github.app_private_key_base64, or ci.github.private_key_path")
			}
		}
	}

	// Check for mutually exclusive configuration
	if cfg.CI.GitHub.AppPrivateKey != "" && cfg.CI.GitHub.AppPrivateKeyBase64 != "" {
		validationErrors = append(validationErrors, "ci.github.app_private_key and ci.github.app_private_key_base64 are mutually exclusive")
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("server configuration validation failed: %s", strings.Join(validationErrors, "; "))
	}

	return nil
}

// formatValidationError converts a validator.FieldError into a readable error message
func formatValidationError(e validator.FieldError) string {
	field := e.Field()
	tag := e.Tag()
	param := e.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "required_if":
		return fmt.Sprintf("%s is required when %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, param)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, param)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, param)
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, param)
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, param)
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, param)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "hostname":
		return fmt.Sprintf("%s must be a valid hostname", field)
	default:
		return fmt.Sprintf("%s failed validation for tag %s with param %s", field, tag, param)
	}
}
