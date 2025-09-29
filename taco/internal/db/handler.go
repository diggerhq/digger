package db

import (
	"log"
	"time"
	"gorm.io/gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm/logger"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	rbac "github.com/diggerhq/digger/opentaco/internal/rbac"
	"context"
	"os"
	"path/filepath"

)


type Role struct {
	ID int64 `gorm:"primaryKey"`
	RoleId string	`gorm:"not null;uniqueIndex"`// like "admin"
	Name string //" admin role"
	Description string // "Admin Role with full access"
	Permissions []Permission  `gorm:"many2many:role_permissions;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt time.Time//timestamp
	CreatedBy string //subject of creator (self for admin)
}




type Permission struct {
	ID int64	`gorm:"primaryKey"`
	PermissionId string	`gorm:"not null;uniqueIndex"`
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




type User struct {
	ID int64	`gorm:"primaryKey"`
	Subject string	`gorm:"not null;uniqueIndex"`
	Email string	`gorm:"not nulll;uniqueIndex"`
	Roles []Role	`gorm:"many2many:user_roles;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Version int64 //"1"
}

type Unit struct { 
	ID int64	`gorm:"primaryKey"`
	Name string		`gorm:"uniqueIndex"`
	Tags []Tag  `gorm:"many2many:unit_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`

}

type Tag struct {
	ID int64	`gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`

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
/*

todo 

ingest s3 
make adapter so this can be used 
make UNIT LS look up with this sytem in the adapter as simple POC


*/ 


var DefaultModels = []any{
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

type DBConfig struct {
	Path string
	Models []any 
}


func OpenSQLite(cfg DBConfig) *gorm.DB {

	if cfg.Path == "" {
		cfg.Path = "./data/taco.db"

	
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		log.Fatalf("create db dir: %v", err)
	}



	}
	if len(cfg.Models) == 0 { cfg.Models = DefaultModels }

	// Keep DSN simple; set PRAGMAs via Exec (works reliably across drivers).
	dsn := "file:" + cfg.Path + "?cache=shared"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // show SQL while developing
	})
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}

	// Connection pool hints (SQLite is single-writer; 1 open conn is safe)
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("unwrap sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	// Helpful PRAGMAs
	if err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA foreign_keys=ON;
		PRAGMA busy_timeout=5000;
	`).Error; err != nil {
		log.Fatalf("pragmas: %v", err)
	}

	// AutoMigrate your models (add them below or pass via args)
	if err := db.AutoMigrate(cfg.Models...); err != nil {
			log.Fatalf("automigrate: %v", err)
	}

	    // Create the user-unit access view for fast ls-fast lookups
		if err := db.Exec(`
        CREATE VIEW IF NOT EXISTS user_unit_access AS
        WITH user_permissions AS (
            SELECT DISTINCT 
                u.subject as user_subject,
                r.id as rule_id,
                r.wildcard_resource,
                r.effect
            FROM users u
            JOIN user_roles ur ON u.id = ur.user_id  
            JOIN role_permissions rp ON ur.role_id = rp.role_id
            JOIN rules r ON rp.permission_id = r.permission_id
            LEFT JOIN rule_actions ra ON r.id = ra.rule_id
            WHERE r.effect = 'allow'
              AND (r.wildcard_action = 1 OR ra.action = 'unit.read' OR ra.action IS NULL)
        ),
        wildcard_access AS (
            SELECT DISTINCT 
                up.user_subject,
                un.name as unit_name
            FROM user_permissions up
            CROSS JOIN units un
            WHERE up.wildcard_resource = 1
        ),
        specific_access AS (
            SELECT DISTINCT 
                up.user_subject,
                un.name as unit_name
            FROM user_permissions up
            JOIN rule_units ru ON up.rule_id = ru.rule_id
            JOIN units un ON ru.unit_id = un.id
            WHERE up.wildcard_resource = 0
        )
        SELECT user_subject, unit_name FROM wildcard_access
        UNION 
        SELECT user_subject, unit_name FROM specific_access;
    `).Error; err != nil {
        log.Printf("Warning: failed to create user_unit_access view: %v", err)
    }
    
	
	return db
}






// should make an adapter for this process, but for POC just s3store 
func Seed(ctx context.Context, store storage.S3Store, db *gorm.DB){ 
    

	//gets called from service boot

	// call store 
	//for each document location
	//get all the units TODO: consider tags
	allUnits, err := store.List(ctx, "")
	
	if err != nil {
    	log.Fatal(err)
	}

	//go through each unit 
	// should batch or use iter for scale - but proof of concept
	// pagination via s3store would be trivial 
	for _, unit := range allUnits {
		// create records 
		r := Unit{Name: unit.ID}
		if err := db.FirstOrCreate(&r, Unit{Name: unit.ID}).Error; err != nil {
		    // if existed, r is loaded; else it’s created
		    log.Printf("Failed to create or find unit  %s: %v", unit.ID, err)
		    continue
		}
	}

	// Right now there is no RBAC adapter either, outside of POC should actually implement this as well 
	S3RBACStore := rbac.NewS3RBACStore(store.GetS3Client(), store.GetS3Bucket(), store.GetS3Prefix())



	//permission
	permissions, err := S3RBACStore.ListPermissions(ctx)
	if err != nil {
		log.Fatal(err) 
	}
	for _, permission := range permissions {
		err := SeedPermission(ctx, db, permission)
		if err != nil{
			log.Printf("Failed to seed permission: %s", permission.ID)
			continue
		}
	}
	

	//roles 
	roles, err := S3RBACStore.ListRoles(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, role := range roles {
		err := SeedRole(ctx, db, role)
		if err != nil {
			log.Printf("Failed to seed role: %s", role.ID)
			continue
		}
	}



	//users
	users, err := S3RBACStore.ListUserAssignments(ctx)
	if err != nil {
		log.Fatal(err) 
	}
	for _, user := range users {
		err := SeedUser(ctx,db,user) 
		if err != nil {
			log.Printf("Failed to seed user: %s", user.Subject)
			continue
		}

	}


	//TBD
	//TFE tokens. 
	//system id section
	//audit logs 
	//etc 



}
