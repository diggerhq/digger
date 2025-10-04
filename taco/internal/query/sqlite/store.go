package sqlite


import (
	"os"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"fmt"
	"context"
	"log"
	"time"


	"github.com/diggerhq/digger/opentaco/internal/query/types"

)


type SQLiteQueryStore struct { 
	db *gorm.DB
	config Config
}


type Config struct {
	Path string
	Models  []any
	Cache string
	EnableForeignKeys bool 
	EnableWAL bool
	BusyTimeout time.Duration
	MaxOpenConns int
	MaxIdleConns int 
	ConnMaxLifetime time.Duration
}

func NewSQLiteQueryStore(cfg Config) (*SQLiteQueryStore, error) {

	//set up SQLite 
	db, err := openSQLite(cfg) 

	if err != nil {

		return nil, fmt.Errorf("Failed to open SQLite: %s", err)
	}

	//initialize the store
	store := &SQLiteQueryStore{db: db, config: cfg}

	
	// migrate the models
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("Failed to migrate store: %w", err)
	}

	// create the views for the store
	if err := store.createViews(); err != nil { 
		return nil, fmt.Errorf("Failed to create views for the store: %v", err)
	}

	log.Printf("SQLite query store successfully initialized: %s", cfg.Path)
	

	return store, nil
}



func openSQLite(cfg Config) (*gorm.DB, error){ 


	if cfg.Path == "" {
		cfg.Path = "./data/taco.db"

		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755);  err != nil {
			return nil, fmt.Errorf("create db dir: %v", err)
		}

	}


	if cfg.Cache == "" {
		cfg.Cache = "shared"
	}

	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = 5 * time.Second
	}

	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 1 
	}

	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 1
	}


	if cfg.PragmaJournalMode == ""{
		cfg.PragmaJournalMode = "WAL"
	}

	if cfg.PragmaForeignKeys == "" {
		cfg.PragmaForeignKeys = "ON"
	}

	if cfg.PragmaBusyTimeout == "" {
		cfg.PragmaBusyTimeout = "5000"
	}

    // (ConnMaxLifeTime default to 0)


	dsn := fmt.Sprintf ("file:%s?cache=%v", cfg.Path, cfg.Cache)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // show SQL while developing
	})
	if err != nil {
		return nil , fmt.Errorf("open sqlite: %v", err)
	}

	// Connection pool hints (SQLite is single-writer; 1 open conn is safe)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("unwrap sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)


	if err := db.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", strings.ToUpper(cfg.PragmaJournalMode))).Error; err != nil {
    	return nil, fmt.Errorf("apply journal_mode: %w", err)
	}
	
	if err := db.Exec(fmt.Sprintf("PRAGMA foreign_keys = %s;", strings.ToUpper(cfg.PragmaForeignKeys))).Error; err != nil {
    	return nil, fmt.Errorf("apply foreign_keys: %w", err)
	}
	
	if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %s;", cfg.PragmaBusyTimeout)).Error; err != nil {
    	return nil, fmt.Errorf("apply busy_timeout: %w", err)
	}

	return db, nil

}


func (s *SQLiteQueryStore) migrate() error {
	
	// expect default models 
	models := types.DefaultModels


	// if the models are specified, load them 
	if len(s.config.Models) > 0 {
		models = s.config.Models 
	}

	if err := s.db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("Migration failed: %w", err)
	}

	return nil 
}


// prefix is the location within the bucket like /prod/region1/etc 
func (s *SQLiteQueryStore) ListUnits(ctx context.Context, prefix string) ([]types.Unit, error) {
    var units []types.Unit 
    q := s.db.WithContext(ctx).Preload("Tags")
    
    if prefix != "" {
        q = q.Where("name LIKE ?", prefix+"%")
    }
    
    if err := q.Find(&units).Error; err != nil {
        return nil, err
    }
    
    return units, nil  
}

func (s *SQLiteQueryStore) GetUnit(ctx context.Context, id string) (*types.Unit, error) {
    var unit types.Unit
    err := s.db.WithContext(ctx).
        Preload("Tags").
        Where("name = ?", id).
        First(&unit).Error
    
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, types.ErrNotFound
        }
        return nil, err
    }
    
    return &unit, nil  
}




func (s *SQLiteQueryStore) IsEnabled() bool {
	// not NOOP ? 
	return true
}

func (s *SQLiteQueryStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close() 
}



func (s *SQLiteQueryStore) SyncEnsureUnit(ctx context.Context, unitName string) error {
    unit := types.Unit{Name: unitName}
    return s.db.WithContext(ctx).FirstOrCreate(&unit, types.Unit{Name: unitName}).Error
}

func (s *SQLiteQueryStore) SyncDeleteUnit(ctx context.Context, unitName string) error {
    return s.db.WithContext(ctx).Where("name = ?", unitName).Delete(&types.Unit{}).Error
}






func (s *SQLiteQueryStore) FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error) {
	
// empty input?
	if len(unitIDs) == 0  {
		return []string{}, nil 
	}


	var allowedUnitIDs []string



	err := s.db.WithContext(ctx).
	Table("user_unit_access").
	Select("unit_name").
	Where ("user_subject =  ?", userSubject).
	Where("unit_name IN ?", unitIDs).
	Pluck("unit_name", &allowedUnitIDs).Error


	if err != nil {
		return nil, fmt.Errorf("Failed to filter the units by user : %w", err) 

	}

	return allowedUnitIDs, nil 
}




func (s *SQLiteQueryStore) ListUnitsForUser(ctx context.Context, userSubject string, prefix string) ([]types.Unit, error) {
    var units []types.Unit
    
    q := s.db.WithContext(ctx).
        Table("units").
        Select("units.*").
        Joins("JOIN user_unit_access ON units.name = user_unit_access.unit_name").
        Where("user_unit_access.user_subject = ?", userSubject).
        Preload("Tags")
        
        if prefix != "" {
        	q = q.Where("units.name LIKE ?", prefix +"%")
        }



        err:= q.Find(&units).Error
    
    if err != nil {
        return nil, fmt.Errorf("failed to list units for user: %w", err)
    }
    
    return units, nil
}


type viewDefinition struct {
    name string
    sql  string
}
// CTE method for user_permissions - gets users with their permission rules
func (s *SQLiteQueryStore) userPermissionsCTE() string {
    return `
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
          AND (r.wildcard_action = 1 OR ra.action = 'unit.read' OR ra.action IS NULL)`
}

// CTE method for wildcard_access - handles users with wildcard resource access
func (s *SQLiteQueryStore) wildcardAccessCTE() string {
    return `
        SELECT DISTINCT 
            up.user_subject,
            un.name as unit_name
        FROM user_permissions up
        CROSS JOIN units un
        WHERE up.wildcard_resource = 1`
}

// CTE method for specific_access - handles users with specific unit access
func (s *SQLiteQueryStore) specificAccessCTE() string {
    return `
        SELECT DISTINCT 
            up.user_subject,
            un.name as unit_name
        FROM user_permissions up
        JOIN rule_units ru ON up.rule_id = ru.rule_id
        JOIN units un ON ru.unit_id = un.id
        WHERE up.wildcard_resource = 0`
}

// Helper method to create individual views
func (s *SQLiteQueryStore) createView(name, sql string) error {
    query := fmt.Sprintf("CREATE VIEW IF NOT EXISTS %s AS %s", name, sql)
    return s.db.Exec(query).Error
}

// Refactored createViews method
func (s *SQLiteQueryStore) createViews() error {
    views := []viewDefinition{
        {
            name: "user_unit_access",
            sql:  s.buildUserUnitAccessView(),
        },
    }
    
    for _, view := range views {
        if err := s.createView(view.name, view.sql); err != nil {
            return fmt.Errorf("failed to create view %s: %w", view.name, err)
        }
    }
    return nil
}

// Refactored buildUserUnitAccessView method
func (s *SQLiteQueryStore) buildUserUnitAccessView() string {
    return `
        WITH user_permissions AS (` + s.userPermissionsCTE() + `),
             wildcard_access AS (` + s.wildcardAccessCTE() + `),
             specific_access AS (` + s.specificAccessCTE() + `)
        SELECT user_subject, unit_name FROM wildcard_access
        UNION 
        SELECT user_subject, unit_name FROM specific_access`
}

