package types

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role struct {
	ID          string        `gorm:"type:varchar(36);primaryKey"`
	OrgID       string        `gorm:"type:varchar(36);index"` // Foreign key to organizations.id (UUID)
	Name        string        `gorm:"type:varchar(255);not null;index"` // Unique identifier (e.g., "admin", "viewer")
	Description string
	Permissions []Permission  `gorm:"many2many:role_permissions;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
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
	OrgID       string `gorm:"type:varchar(36);index"` // Foreign key to organizations.id (UUID)
	Name        string `gorm:"type:varchar(255);not null;index"` // Unique identifier (e.g., "unit-read", "unit-write")
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
	ID               string `gorm:"type:varchar(36);primaryKey"`
	PermissionID     string `gorm:"type:varchar(36);index;not null"`
	Effect           string `gorm:"size:8;not null;default:allow"`
	WildcardAction   bool   `gorm:"not null;default:false"`
	WildcardResource bool   `gorm:"not null;default:false"`
	ResourcePatterns string `gorm:"type:text;"`
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
	ID          string `gorm:"type:varchar(36);primaryKey"`
	Name        string `gorm:"type:varchar(255);not null;uniqueIndex"` // Unique identifier (e.g., "acme") - used in CLI and paths
	DisplayName string `gorm:"type:varchar(255);not null"`             // Friendly name (e.g., "Acme Corp") - shown in UI
	CreatedBy   string `gorm:"type:varchar(255);not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	ID          string     `gorm:"type:varchar(36);primaryKey"`
	OrgID       string     `gorm:"type:varchar(36);index"` // Foreign key to organizations.id (UUID)
	Name        string     `gorm:"type:varchar(255);not null;index"`
	Size        int64      `gorm:"default:0"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
	Locked      bool       `gorm:"default:false"`
	LockID      string     `gorm:"default:''"`
	LockWho     string     `gorm:"default:''"`
	LockCreated *time.Time
	Tags        []Tag      `gorm:"many2many:unit_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
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
	OrgID string `gorm:"type:varchar(36);index"` // Foreign key to organizations.id (UUID)
	Name  string `gorm:"type:varchar(255);not null;index"`
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
}