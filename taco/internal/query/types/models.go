package types

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Role struct {
	ID          string `gorm:"type:varchar(36);primaryKey"`
	OrgID       string `gorm:"type:varchar(36);index;index:idx_roles_org_id_name;uniqueIndex:unique_org_role_name"`           // Foreign key to organizations.id (UUID)
	Name        string `gorm:"type:varchar(255);not null;index;index:idx_roles_org_id_name;uniqueIndex:unique_org_role_name"` // Composite index for (org_id, name) queries
	Description string
	Permissions []Permission `gorm:"many2many:role_permissions;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt   time.Time
	CreatedBy   string
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

func (Role) TableName() string { return "roles" }

type Permission struct {
	ID          string `gorm:"type:varchar(36);primaryKey"`
	OrgID       string `gorm:"type:varchar(36);index;index:idx_permissions_org_id_name;uniqueIndex:unique_org_permission_name"`           // Foreign key to organizations.id (UUID)
	Name        string `gorm:"type:varchar(255);not null;index;index:idx_permissions_org_id_name;uniqueIndex:unique_org_permission_name"` // Composite index for (org_id, name) queries
	Description string
	Rules       []Rule `gorm:"constraint:OnDelete:CASCADE"`
	CreatedBy   string
	CreatedAt   time.Time
}

func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

func (Permission) TableName() string { return "permissions" }

type Rule struct {
	ID               string        `gorm:"type:varchar(36);primaryKey"`
	PermissionID     string        `gorm:"type:varchar(36);index;not null"`
	Effect           string        `gorm:"size:8;not null;default:allow"`
	WildcardAction   bool          `gorm:"not null;default:false"`
	WildcardResource bool          `gorm:"not null;default:false"`
	ResourcePatterns string        `gorm:"type:text;"`
	Actions          []RuleAction  `gorm:"constraint:OnDelete:CASCADE"`
	UnitTargets      []RuleUnit    `gorm:"constraint:OnDelete:CASCADE"`
	TagTargets       []RuleUnitTag `gorm:"constraint:OnDelete:CASCADE"`
}

func (r *Rule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

type RuleAction struct {
	ID     string `gorm:"type:varchar(36);primaryKey"`
	RuleID string `gorm:"type:varchar(36);index;not null"`
	Action string `gorm:"size:128;not null;index"`
}

func (RuleAction) TableName() string { return "rule_actions" }

func (ra *RuleAction) BeforeCreate(tx *gorm.DB) error {
	if ra.ID == "" {
		ra.ID = uuid.New().String()
	}
	return nil
}

type RuleUnit struct {
	ID     string `gorm:"type:varchar(36);primaryKey"`
	RuleID string `gorm:"type:varchar(36);index;not null"`
	UnitID string `gorm:"type:varchar(36);index;not null"`
}

func (RuleUnit) TableName() string { return "rule_units" }

func (ru *RuleUnit) BeforeCreate(tx *gorm.DB) error {
	if ru.ID == "" {
		ru.ID = uuid.New().String()
	}
	return nil
}

type RuleUnitTag struct {
	ID     string `gorm:"type:varchar(36);primaryKey"`
	RuleID string `gorm:"type:varchar(36);index;not null"`
	TagID  string `gorm:"type:varchar(36);index;not null"`
}

func (RuleUnitTag) TableName() string { return "rule_unit_tags" }

func (rut *RuleUnitTag) BeforeCreate(tx *gorm.DB) error {
	if rut.ID == "" {
		rut.ID = uuid.New().String()
	}
	return nil
}

type Organization struct {
	ID            string  `gorm:"type:varchar(36);primaryKey"`
	Name          string  `gorm:"type:varchar(255);not null;index"` // Non-unique - multiple orgs can have same name (e.g., "Personal")
	DisplayName   string  `gorm:"type:varchar(255);not null"`       // Friendly name (e.g., "Acme Corp") - shown in UI
	ExternalOrgID *string `gorm:"type:varchar(500);uniqueIndex"`    // External org identifier (optional, nullable) - THIS is unique
	CreatedBy     string  `gorm:"type:varchar(255);not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

type User struct {
	ID        string `gorm:"type:varchar(36);primaryKey"`
	Subject   string `gorm:"type:varchar(255);not null;uniqueIndex"`
	Email     string `gorm:"type:varchar(255);not null;uniqueIndex"`
	Roles     []Role `gorm:"many2many:user_roles;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int64
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

type Unit struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	OrgID       string    `gorm:"type:varchar(36);index;index:idx_units_org_id_name;uniqueIndex:idx_units_org_name"`           // Foreign key to organizations.id (UUID)
	Name        string    `gorm:"type:varchar(255);not null;index;index:idx_units_org_id_name;uniqueIndex:idx_units_org_name"` // Composite index for (org_id, name) queries
	Size        int64     `gorm:"default:0"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
	Locked      bool      `gorm:"default:false"`
	LockID      string    `gorm:"default:''"`
	LockWho     string    `gorm:"default:''"`
	LockCreated *time.Time
	Tags        []Tag `gorm:"many2many:unit_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`

	// TFE workspace settings (nullable for non-TFE usage)
	TFEAutoApply        *bool   `gorm:"default:null"`
	TFETerraformVersion *string `gorm:"type:varchar(50);default:null"`
	TFEEngine           *string `gorm:"type:varchar(20);default:'terraform'"` // 'terraform' or 'tofu'
	TFEWorkingDirectory *string `gorm:"type:varchar(500);default:null"`
	TFEExecutionMode    *string `gorm:"type:varchar(50);default:null"` // 'remote', 'local', 'agent'
}

func (u *Unit) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

func (Unit) TableName() string { return "units" }

type Tag struct {
	ID    string `gorm:"type:varchar(36);primaryKey"`
	OrgID string `gorm:"type:varchar(36);index;index:idx_tags_org_id_name;uniqueIndex:unique_org_tag_name"`           // Foreign key to organizations.id (UUID)
	Name  string `gorm:"type:varchar(255);not null;index;index:idx_tags_org_id_name;uniqueIndex:unique_org_tag_name"` // Composite index for (org_id, name) queries
}

func (t *Tag) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

func (Tag) TableName() string { return "tags" }

type UnitTag struct {
	UnitID string `gorm:"type:varchar(36);primaryKey;index"`
	TagID  string `gorm:"type:varchar(36);primaryKey;index"`
}

func (UnitTag) TableName() string { return "unit_tags" }

type UserRole struct {
	UserID string `gorm:"type:varchar(36);primaryKey;index"`
	RoleID string `gorm:"type:varchar(36);primaryKey;index"`
	OrgID  string `gorm:"type:varchar(36);primaryKey;index"`
}

func (UserRole) TableName() string { return "user_roles" }

type RolePermission struct {
	RoleID       string `gorm:"type:varchar(36);primaryKey;index"`
	PermissionID string `gorm:"type:varchar(36);primaryKey;index"`
}

func (RolePermission) TableName() string { return "role_permissions" }

type Token struct {
	ID         string `gorm:"type:varchar(36);primaryKey"`
	UserID     string `gorm:"type:varchar(255);index;not null"` // Flexible for external user IDs
	OrgID      string `gorm:"type:varchar(255);index;not null"` // Flexible for external org IDs
	Token      string `gorm:"type:varchar(255);uniqueIndex;not null"`
	Name       string `gorm:"type:varchar(255)"`
	Status     string `gorm:"type:varchar(20);default:active"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

func (Token) TableName() string { return "tokens" }

// TFE Run model - represents a Terraform run (plan/apply execution)
type TFERun struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	OrgID     string    `gorm:"type:varchar(36);index;not null"`
	UnitID    string    `gorm:"type:varchar(36);not null;index"` // FK to units.id (the workspace)
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// TFE-specific attributes
	Status    string `gorm:"type:varchar(50);not null;default:'pending'"`
	IsDestroy bool   `gorm:"default:false"`
	Message   string `gorm:"type:text"`
	PlanOnly  bool   `gorm:"default:true"`
	AutoApply bool   `gorm:"default:false"`                  // Whether to auto-trigger apply after successful plan
	Source    string `gorm:"type:varchar(50);default:'cli'"` // 'cli', 'api', 'ui', 'vcs'

	// Actions (stored as fields)
	IsCancelable bool `gorm:"default:true"`
	CanApply     bool `gorm:"default:false"`

	// Relationships (foreign keys)
	ConfigurationVersionID string  `gorm:"type:varchar(36);not null;index"`
	PlanID                 *string `gorm:"type:varchar(36);index"` // Nullable until plan is created
	ApplyID                *string `gorm:"type:varchar(36);index"` // Nullable if plan-only

	// Blob storage references
	ApplyLogBlobID *string `gorm:"type:varchar(255)"` // Blob ID for apply logs

	// Error tracking
	ErrorMessage *string `gorm:"type:text"` // Stores error message if run fails

	// Metadata
	CreatedBy string `gorm:"type:varchar(255)"`

	// Associations
	Unit                 *Unit                    `gorm:"foreignKey:UnitID"`
	Plan                 *TFEPlan                 `gorm:"foreignKey:PlanID"`
	ConfigurationVersion *TFEConfigurationVersion `gorm:"foreignKey:ConfigurationVersionID"`
}

func (r *TFERun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		// Generate TFE-style ID: run-{uuid} (full UUID for compatibility)
		r.ID = "run-" + strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	return nil
}

func (TFERun) TableName() string { return "tfe_runs" }

// TFE Plan model - represents a Terraform plan execution
type TFEPlan struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	OrgID     string    `gorm:"type:varchar(36);index;not null"`
	RunID     string    `gorm:"type:varchar(36);not null;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// Plan attributes
	Status               string `gorm:"type:varchar(50);not null;default:'pending'"`
	ResourceAdditions    int    `gorm:"default:0"`
	ResourceChanges      int    `gorm:"default:0"`
	ResourceDestructions int    `gorm:"default:0"`
	HasChanges           bool   `gorm:"default:false"`

	// Log storage - reference to blob storage
	LogBlobID  *string `gorm:"type:varchar(255)"` // Reference to blob storage key
	LogReadURL *string `gorm:"type:text"`         // Signed URL for log access (temporary)

	// Plan output/data stored in blob storage or as JSON
	PlanOutputBlobID *string `gorm:"type:varchar(255)"` // Reference to blob storage for large plans
	PlanOutputJSON   *string `gorm:"type:longtext"`     // Inline JSON for smaller plans

	// Metadata
	CreatedBy string `gorm:"type:varchar(255)"`

	// Associations
	Run *TFERun `gorm:"foreignKey:RunID"`
}

func (p *TFEPlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		// Generate TFE-style ID: plan-{uuid} (full UUID for compatibility)
		p.ID = "plan-" + strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	return nil
}

func (TFEPlan) TableName() string { return "tfe_plans" }

// TFE Configuration Version model - represents an uploaded Terraform configuration
type TFEConfigurationVersion struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	OrgID     string    `gorm:"type:varchar(36);index;not null"`
	UnitID    string    `gorm:"type:varchar(36);not null;index"` // FK to units.id (the workspace)
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// Configuration version attributes
	Status        string `gorm:"type:varchar(50);not null;default:'pending'"`
	Source        string `gorm:"type:varchar(50);default:'cli'"` // 'cli', 'api', 'vcs', 'terraform'
	Speculative   bool   `gorm:"default:false"`                  // false = normal apply, true = plan-only
	AutoQueueRuns bool   `gorm:"default:false"`
	Provisional   bool   `gorm:"default:false"`

	// Error handling
	Error        *string `gorm:"type:text"`
	ErrorMessage *string `gorm:"type:text"`

	// Upload handling
	UploadURL     *string    `gorm:"type:text"` // Signed upload URL (temporary)
	UploadedAt    *time.Time // When upload completed
	ArchiveBlobID *string    `gorm:"type:varchar(255)"` // Reference to stored archive in blob storage

	// Status timestamps stored as JSON
	StatusTimestamps string `gorm:"type:json;default:'{}'"` // JSON map of status -> timestamp

	// Metadata
	CreatedBy string `gorm:"type:varchar(255)"`

	// Associations
	Unit *Unit    `gorm:"foreignKey:UnitID"`
	Runs []TFERun `gorm:"foreignKey:ConfigurationVersionID"`
}

func (cv *TFEConfigurationVersion) BeforeCreate(tx *gorm.DB) error {
	if cv.ID == "" {
		// Generate TFE-style ID: cv-{uuid} (full UUID for compatibility)
		cv.ID = "cv-" + strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	if cv.StatusTimestamps == "" {
		cv.StatusTimestamps = "{}"
	}
	return nil
}

func (TFEConfigurationVersion) TableName() string { return "tfe_configuration_versions" }

// RemoteRunActivityModel captures sandbox execution stats for billing
type RemoteRunActivity struct {
	ID              string  `gorm:"type:varchar(36);primaryKey"`
	RunID           string  `gorm:"type:varchar(36);index;not null"`
	OrgID           string  `gorm:"type:varchar(36);index;not null"`
	UnitID          string  `gorm:"type:varchar(36);index;not null"`
	Operation       string  `gorm:"type:varchar(16);not null"`
	Status          string  `gorm:"type:varchar(32);not null;default:'pending'"`
	TriggeredBy     string  `gorm:"type:varchar(255)"`
	TriggeredSource string  `gorm:"type:varchar(50)"`
	SandboxProvider string  `gorm:"type:varchar(50)"`
	SandboxJobID    *string `gorm:"type:varchar(100)"`
	StartedAt       *time.Time
	CompletedAt     *time.Time
	DurationMs      *int64    `gorm:"type:bigint"`
	ErrorMessage    *string   `gorm:"type:text"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (rra *RemoteRunActivity) BeforeCreate(tx *gorm.DB) error {
	if rra.ID == "" {
		rra.ID = uuid.New().String()
	}
	return nil
}

func (RemoteRunActivity) TableName() string { return "remote_run_activity" }

var DefaultModels = []any{
	&Organization{},
	&User{},
	&Role{},
	&UserRole{},
	&Permission{},
	&Rule{},
	&RuleAction{},
	&RuleUnit{},
	&RuleUnitTag{},
	&RolePermission{},
	&Unit{},
	&Tag{},
	&UnitTag{},
	&Token{},
	&TFERun{},
	&TFEPlan{},
	&TFEConfigurationVersion{},
	&RemoteRunActivity{},
}
