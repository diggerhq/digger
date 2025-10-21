package common

import (
	"fmt"
	"log"
	"gorm.io/gorm"
)

func CreateOrgScopedIndexes(db *gorm.DB) error {
	dialect := db.Dialector.Name()
	
	indexes := []struct {
		table   string
		name    string
		columns string
	}{
		{"units", "idx_units_org_name", "org_id, name"},
		{"roles", "idx_roles_org_role_id", "org_id, role_id"},
		{"permissions", "idx_permissions_org_permission_id", "org_id, permission_id"},
		{"tags", "idx_tags_org_name", "org_id, name"},
	}
	
	for _, idx := range indexes {
		if err := createUniqueIndexIfNotExists(db, dialect, idx.table, idx.name, idx.columns); err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.name, err)
		}
		log.Printf("Ensured unique index %s on %s(%s)", idx.name, idx.table, idx.columns)
	}
	
	return nil
}

func createUniqueIndexIfNotExists(db *gorm.DB, dialect, table, indexName, columns string) error {
	switch dialect {
	case "sqlite":
		sql := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s(%s)", indexName, table, columns)
		return db.Exec(sql).Error
		
	case "postgres":
		sql := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s(%s)", indexName, table, columns)
		return db.Exec(sql).Error
		
	case "mysql":
		var count int64
		checkSQL := fmt.Sprintf(`
			SELECT COUNT(*) FROM information_schema.statistics 
			WHERE table_schema = DATABASE() 
			AND table_name = '%s' 
			AND index_name = '%s'
		`, table, indexName)
		
		if err := db.Raw(checkSQL).Scan(&count).Error; err != nil {
			return err
		}
		
		if count == 0 {
			sql := fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s(%s)", indexName, table, columns)
			return db.Exec(sql).Error
		}
		
	case "mssql", "sqlserver":
		checkSQL := fmt.Sprintf(`
			IF NOT EXISTS (
				SELECT * FROM sys.indexes 
				WHERE name = '%s' AND object_id = OBJECT_ID('%s')
			)
			CREATE UNIQUE INDEX %s ON %s(%s)
		`, indexName, table, indexName, table, columns)
		return db.Exec(checkSQL).Error
	}
	
	return nil
}

