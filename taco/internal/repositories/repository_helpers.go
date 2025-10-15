package repositories

import "gorm.io/gorm"

// dbProvider interface for extracting GORM DB from query store
// This is shared across all repository implementations
type dbProvider interface {
	GetDB() *gorm.DB
}

// GetDBFromQueryStore extracts the GORM database from a query store
// Returns nil if the query store doesn't provide a database
// This eliminates duplication across all repository constructors
func GetDBFromQueryStore(queryStore interface{}) *gorm.DB {
	provider, ok := queryStore.(dbProvider)
	if !ok {
		return nil
	}
	return provider.GetDB()
}

