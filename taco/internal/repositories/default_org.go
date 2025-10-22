package repositories

import (
	"context"
	"errors"
	"log"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// EnsureDefaultOrganization ensures a default organization exists for self-hosted deployments
// This is idempotent and safe to call multiple times
// Returns the UUID of the default organization
func EnsureDefaultOrganization(ctx context.Context, db *gorm.DB) (string, error) {
	const defaultOrgID = "default"
	const defaultOrgName = "Default Organization"
	
	// Check if default org exists
	var org types.Organization
	err := db.WithContext(ctx).Where("name = ?", defaultOrgID).First(&org).Error
	
	if err == nil {
		// Default org already exists
		log.Printf("Default organization already exists: ID=%s, Name=%s", org.ID, org.Name)
		return org.ID, nil
	}
	
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	
	// Create default org
	defaultOrg := &types.Organization{
		Name:        defaultOrgID,
		DisplayName: defaultOrgName,
		CreatedBy:   "system",
	}
	
	if err := db.WithContext(ctx).Create(defaultOrg).Error; err != nil {
		return "", err
	}
	
	log.Printf("Created default organization: ID=%s, Name=%s, DisplayName=%s", 
		defaultOrg.ID, defaultOrg.Name, defaultOrg.DisplayName)
	return defaultOrg.ID, nil
}

// GetDefaultOrgUUID returns the UUID of the default organization
func GetDefaultOrgUUID(ctx context.Context, db *gorm.DB) (string, error) {
	var org types.Organization
	err := db.WithContext(ctx).Where("name = ?", "default").First(&org).Error
	if err != nil {
		return "", err
	}
	return org.ID, nil
}

