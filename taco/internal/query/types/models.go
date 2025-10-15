package types


import (
	"time"
)


type Role struct {
	ID int64 `gorm:"primaryKey"`
	RoleId string	`gorm:"type:varchar(255);not null;uniqueIndex"`// like "admin"
	Name string //" admin role"
	Description string // "Admin Role with full access"
	Permissions []Permission  `gorm:"many2many:role_permissions;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt time.Time//timestamp
	CreatedBy string //subject of creator (self for admin)
}




type Permission struct {
	ID int64	`gorm:"primaryKey"`
	PermissionId string	`gorm:"type:varchar(255);not null;uniqueIndex"`
	Name string // "admin permission"
	Description string // "Admin permission allowing all action"
	Rules []Rule	`gorm:"constraint:OnDelete:CASCADE"`						// [{"actions":["unit.read","unit.write","unit.lock","unit.delete","rbac.manage"],"resources":["*"],"effect":"allow"}]  FK 
	CreatedBy string // subject of creator (self for admin)
	CreatedAt time.Time
} 

type Rule struct { 
	ID               int64  `gorm:"primaryKey"`
	PermissionID     int64  `gorm:"index;not null"`
	Effect           string `gorm:"size:8;not null;default:allow"` // "allow" | "deny"
	WildcardAction   bool   `gorm:"not null;default:false"`
	WildcardResource bool   `gorm:"not null;default:false"`
	ResourcePatterns string `gorm:"type:text;default:''"` // JSON array of resource patterns like ["dev/*", "staging/*"]
	Actions          []RuleAction   `gorm:"constraint:OnDelete:CASCADE"`
	UnitTargets  	 []RuleUnit `gorm:"constraint:OnDelete:CASCADE"`
	TagTargets       []RuleUnitTag `gorm:"constraint:OnDelete:CASCADE"`
}



type RuleAction struct {
	ID     int64  `gorm:"primaryKey"`
	RuleID int64  `gorm:"index;not null"`
	Action string `gorm:"size:128;not null;index"`
	// UNIQUE (rule_id, action)
}
func (RuleAction) TableName() string { return "rule_actions" }

type RuleUnit struct {
	ID         int64 `gorm:"primaryKey"`
	RuleID     int64 `gorm:"index;not null"`
	UnitID 	   int64 `gorm:"index;not null"`
	// UNIQUE (rule_id, resource_id)
}
func (RuleUnit) TableName() string { return "rule_units" }

type RuleUnitTag struct {
	ID     int64 `gorm:"primaryKey"`
	RuleID int64 `gorm:"index;not null"`
	TagID  int64 `gorm:"index;not null"`
	// UNIQUE (rule_id, tag_id)
}
func (RuleUnitTag) TableName() string { return "rule_unit_tags" }




type Organization struct {
	ID int64 `gorm:"primaryKey"`
	OrgID string `gorm:"type:varchar(255);not null;uniqueIndex"` // e.g., "acme-corp"
	Name string `gorm:"type:varchar(255);not null"` // e.g., "Acme Corporation"
	CreatedBy string `gorm:"type:varchar(255);not null"` // Subject of creator
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID int64	`gorm:"primaryKey"`
	Subject string	`gorm:"type:varchar(255);not null;uniqueIndex"`
	Email string	`gorm:"type:varchar(255);not null;uniqueIndex"`
	Roles []Role	`gorm:"many2many:user_roles;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Version int64 //"1"
}

type Unit struct { 
	ID        int64     `gorm:"primaryKey"`
	Name      string    `gorm:"type:varchar(255);uniqueIndex"`
	Size      int64     `gorm:"default:0"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	Locked    bool      `gorm:"default:false"`
	LockID    string    `gorm:"default:''"`
	LockWho   string    `gorm:"default:''"`
	LockCreated *time.Time
	Tags      []Tag     `gorm:"many2many:unit_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
}

type Tag struct {
	ID int64	`gorm:"primaryKey"`
	Name string `gorm:"type:varchar(255);uniqueIndex"`

} 


//explicit joins


type UnitTag struct {
	UnitID 		int64 `gorm:"primaryKey;index"`
	TagID      	int64 `gorm:"primaryKey;index"`
}
func (UnitTag) TableName() string { return "unit_tags" }


type UserRole struct {
	UserID int64 `gorm:"primaryKey;index"`
	RoleID int64 `gorm:"primaryKey;index"`
}
func (UserRole) TableName() string { return "user_roles" }



type RolePermission struct { 
	RoleID int64 `gorm:"primaryKey;index"` 
	PermissionID int64 `gorm:"primaryKey;index"` 
}


func (RolePermission) TableName() string { return "role_permissions" }




// set the models that will be populated on startup for each DB type; add any new tables here:
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