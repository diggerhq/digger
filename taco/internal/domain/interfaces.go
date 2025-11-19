package domain

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// ============================================
// Core State Operations
// ============================================

// StateOperations defines the common operations most handlers need.
// This is the practical interface for backend, S3-compat, and TFE handlers.
// Includes read, write, and lock operations but NOT list or admin operations.
type StateOperations interface {
	// Read operations
	Get(ctx context.Context, id string) (*storage.UnitMetadata, error)
	Download(ctx context.Context, id string) ([]byte, error)
	GetLock(ctx context.Context, id string) (*storage.LockInfo, error)

	// Write operations
	Upload(ctx context.Context, id string, data []byte, lockID string) error

	// Lock operations
	Lock(ctx context.Context, id string, info *storage.LockInfo) error
	Unlock(ctx context.Context, id string, lockID string) error
}

// ============================================
// Management Operations
// ============================================

// UnitManagement extends StateOperations with admin/management operations.
// This is for the unit management API that needs full CRUD + versioning.
// All operations are org-scoped in the new architecture.
type UnitManagement interface {
	StateOperations

	// Admin operations (org-scoped)
	Create(ctx context.Context, orgID string, name string) (*storage.UnitMetadata, error)
	List(ctx context.Context, orgID string, prefix string) ([]*storage.UnitMetadata, error)
	Delete(ctx context.Context, id string) error // Uses UUID

	// Version operations (UUID-based)
	ListVersions(ctx context.Context, id string) ([]*storage.VersionInfo, error)
	RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error
}

// ============================================
// TFE-Specific Operations
// ============================================

// TFEOperations defines what TFE handler needs.
// TFE needs read/write/lock operations only
type TFEOperations interface {
	StateOperations
}

// TFERunRepository manages TFE run lifecycle
type TFERunRepository interface {
	// Create a new run
	CreateRun(ctx context.Context, run *TFERun) error

	// Get run by ID
	GetRun(ctx context.Context, runID string) (*TFERun, error)

	// List runs for a unit (workspace)
	ListRunsForUnit(ctx context.Context, unitID string, limit int) ([]*TFERun, error)

	// Update run status
	UpdateRunStatus(ctx context.Context, runID string, status string) error

	// Update run with plan ID
	UpdateRunPlanID(ctx context.Context, runID string, planID string) error

	// Update run status and can_apply together
	UpdateRunStatusAndCanApply(ctx context.Context, runID string, status string, canApply bool) error

	// Update run with error message (when execution fails)
	UpdateRunError(ctx context.Context, runID string, errorMessage string) error
}

// TFEPlanRepository manages TFE plan lifecycle
type TFEPlanRepository interface {
	// Create a new plan
	CreatePlan(ctx context.Context, plan *TFEPlan) error

	// Get plan by ID
	GetPlan(ctx context.Context, planID string) (*TFEPlan, error)

	// Update plan status and results
	UpdatePlan(ctx context.Context, planID string, updates *TFEPlanUpdate) error

	// Get plan by run ID
	GetPlanByRunID(ctx context.Context, runID string) (*TFEPlan, error)
}

// TFEConfigurationVersionRepository manages configuration versions
type TFEConfigurationVersionRepository interface {
	// Create a new configuration version
	CreateConfigurationVersion(ctx context.Context, cv *TFEConfigurationVersion) error

	// Get configuration version by ID
	GetConfigurationVersion(ctx context.Context, cvID string) (*TFEConfigurationVersion, error)

	// Update configuration version status (and optionally the archive blob ID)
	UpdateConfigurationVersionStatus(ctx context.Context, cvID string, status string, uploadedAt *time.Time, archiveBlobID *string) error

	// List configuration versions for a unit (workspace)
	ListConfigurationVersionsForUnit(ctx context.Context, unitID string, limit int) ([]*TFEConfigurationVersion, error)
}

// RemoteRunActivityRepository records compute usage for remote plan/apply executions
type RemoteRunActivityRepository interface {
	CreateActivity(ctx context.Context, activity *RemoteRunActivity) (string, error)
	MarkRunning(ctx context.Context, activityID string, startedAt time.Time, sandboxProvider string) error
	MarkCompleted(ctx context.Context, activityID string, status string, completedAt time.Time, duration time.Duration, sandboxJobID *string, errorMessage *string) error
	
	// Query operations
	ListActivities(ctx context.Context, filters ActivityFilters) ([]*RemoteRunActivity, error)
	GetUsageSummary(ctx context.Context, orgID string, startDate, endDate *time.Time) (*UsageSummary, error)
}

// ActivityFilters for querying remote run activities
type ActivityFilters struct {
	OrgID     string
	UnitID    *string
	Status    *string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

// UsageSummary aggregates remote run usage for billing
type UsageSummary struct {
	TotalRuns        int
	TotalMinutes     float64
	SuccessfulRuns   int
	FailedRuns       int
	ByOperation      map[string]int     // "plan" -> count, "apply" -> count
	ByUnit           map[string]float64 // unit_id -> minutes
	EstimatedCostUSD float64            // Based on minutes * rate
}

// ============================================
// Full Repository Interface
// ============================================

// UnitRepository provides all unit storage and management operations.
// This is the primary interface that concrete repositories implement.
// Handlers receive scoped interfaces (StateOperations, TFEOperations, UnitManagement)
// depending on what operations they need.
type UnitRepository interface {
	UnitManagement
}

// ============================================
// API Response Models
// ============================================
// These types define the API contract for JSON responses.
// They are separate from internal storage types to allow the API format
// to evolve independently from storage implementation.

// Unit represents a Terraform state unit in API responses
type Unit struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	AbsoluteName string    `json:"absolute_name"`
	Size         int64     `json:"size"`
	Updated      time.Time `json:"updated"`
	Locked       bool      `json:"locked"`
	LockInfo     *Lock     `json:"lock_info,omitempty"`
	
	// TFE workspace settings
	TFEAutoApply        *bool   `json:"tfe_auto_apply,omitempty"`
	TFETerraformVersion *string `json:"tfe_terraform_version,omitempty"`
	TFEEngine           *string `json:"tfe_engine,omitempty"`
	TFEWorkingDirectory *string `json:"tfe_working_directory,omitempty"`
	TFEExecutionMode    *string `json:"tfe_execution_mode,omitempty"`
}

// Lock represents a Terraform state lock in API responses
type Lock struct {
	ID      string    `json:"id"`
	Who     string    `json:"who"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

// Version represents a state version in API responses
type Version struct {
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
}

// SortUnitsByID sorts units by their ID for consistent API responses
func SortUnitsByID(units []*Unit) {
	sort.Slice(units, func(i, j int) bool {
		return units[i].ID < units[j].ID
	})
}

// ============================================
// Utility Functions
// ============================================

// ValidateUnitID validates that a unit ID is safe and doesn't contain path traversal
func ValidateUnitID(id string) error {
	normalized := NormalizeUnitID(id)
	if normalized == "" {
		return errors.New("unit ID cannot be empty")
	}
	if strings.Contains(id, "..") {
		return errors.New("unit ID cannot contain '..'")
	}
	return nil
}

// NormalizeUnitID normalizes a unit ID by removing leading/trailing slashes and collapsing multiple slashes
func NormalizeUnitID(id string) string {
	s := strings.TrimSpace(id)
	s = strings.ToLower(s) // Normalize to lowercase for case-insensitivity
	s = strings.Trim(s, "/")

	// Collapse multiple slashes
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}

	return s
}

// DecodeURLPath decodes a URL-encoded path parameter
func DecodeURLPath(encoded string) (string, error) {
	// URL-decode the path (handles %2F -> /)
	decoded := strings.ReplaceAll(encoded, "%2F", "/")
	decoded = strings.ReplaceAll(decoded, "%2f", "/")
	return decoded, nil
}

// DecodeUnitID decodes a URL-encoded unit ID (currently just normalizes)
func DecodeUnitID(encoded string) string {
	return NormalizeUnitID(encoded)
}

// ============================================
// TFE Domain Models
// ============================================

// TFERun represents a Terraform run (plan/apply execution)
type TFERun struct {
	ID                     string
	OrgID                  string
	UnitID                 string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Status                 string
	IsDestroy              bool
	Message                string
	PlanOnly               bool
	AutoApply              bool // Whether to auto-trigger apply after successful plan
	Source                 string
	IsCancelable           bool
	CanApply               bool
	ConfigurationVersionID string
	PlanID                 *string
	ApplyID                *string
	CreatedBy              string
	ApplyLogBlobID         *string
	ErrorMessage           *string // Stores error message if run fails
}

// TFEPlan represents a Terraform plan execution
type TFEPlan struct {
	ID                   string
	OrgID                string
	RunID                string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Status               string
	ResourceAdditions    int
	ResourceChanges      int
	ResourceDestructions int
	HasChanges           bool
	LogBlobID            *string
	LogReadURL           *string
	PlanOutputBlobID     *string
	PlanOutputJSON       *string
	CreatedBy            string
}

// TFEPlanUpdate contains fields that can be updated on a plan
type TFEPlanUpdate struct {
	Status               *string
	ResourceAdditions    *int
	ResourceChanges      *int
	ResourceDestructions *int
	HasChanges           *bool
	LogBlobID            *string
	LogReadURL           *string
	PlanOutputBlobID     *string
	PlanOutputJSON       *string
}

// TFEConfigurationVersion represents an uploaded Terraform configuration
type TFEConfigurationVersion struct {
	ID               string
	OrgID            string
	UnitID           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Status           string
	Source           string
	Speculative      bool
	AutoQueueRuns    bool
	Provisional      bool
	Error            *string
	ErrorMessage     *string
	UploadURL        *string
	UploadedAt       *time.Time
	ArchiveBlobID    *string
	StatusTimestamps string
	CreatedBy        string
}

// RemoteRunActivity tracks remote sandbox executions for billing/auditing
type RemoteRunActivity struct {
	ID              string
	RunID           string
	OrgID           string
	UnitID          string
	Operation       string
	Status          string
	TriggeredBy     string
	TriggeredSource string
	SandboxProvider string
	SandboxJobID    *string
	StartedAt       *time.Time
	CompletedAt     *time.Time
	DurationMS      *int64
	ErrorMessage    *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
