package repositories

import (
	"context"
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"gorm.io/gorm"
)

// gormIdentifierResolver implements domain.IdentifierResolver using GORM
// This is the infrastructure layer implementation - it knows about databases
type gormIdentifierResolver struct {
	db *gorm.DB
}

// NewIdentifierResolver creates an identifier resolver backed by GORM
// Returns domain interface - callers don't know about GORM
func NewIdentifierResolver(db *gorm.DB) domain.IdentifierResolver {
	return &gormIdentifierResolver{db: db}
}

// ResolveOrganization resolves organization identifier to UUID
func (r *gormIdentifierResolver) ResolveOrganization(ctx context.Context, identifier string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database not available")
	}
	
	parsed, err := domain.ParseIdentifier(identifier)
	if err != nil {
		return "", err
	}
	
	// If already a UUID, return it
	if parsed.Type == domain.IdentifierTypeUUID {
		return parsed.UUID, nil
	}
	
	// Try to resolve by internal name first
	var result struct{ ID string }
	err = r.db.WithContext(ctx).
		Table("organizations").
		Select("id").
		Where("name = ?", parsed.Name).
		First(&result).Error
	
	if err == nil {
		return result.ID, nil
	}
	
	// If not found by name, try external org ID
	// This handles cases where someone passes an external ID directly
	err = r.db.WithContext(ctx).
		Table("organizations").
		Select("id").
		Where("external_org_id = ?", parsed.Name).
		First(&result).Error
	
	if err == nil {
		return result.ID, nil
	}
	
	return "", fmt.Errorf("organization not found: %s", parsed.Name)
}

// ResolveUnit resolves unit identifier to UUID within an organization
func (r *gormIdentifierResolver) ResolveUnit(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "units", identifier, orgID)
}

// ResolveRole resolves role identifier to UUID within an organization
func (r *gormIdentifierResolver) ResolveRole(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "roles", identifier, orgID)
}

// ResolvePermission resolves permission identifier to UUID within an organization
func (r *gormIdentifierResolver) ResolvePermission(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "permissions", identifier, orgID)
}

// ResolveTag resolves tag identifier to UUID within an organization
func (r *gormIdentifierResolver) ResolveTag(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "tags", identifier, orgID)
}

// resolveResource is the generic resolver for org-scoped resources
func (r *gormIdentifierResolver) resolveResource(ctx context.Context, table, identifier, orgID string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database not available")
	}
	
	parsed, err := domain.ParseIdentifier(identifier)
	if err != nil {
		return "", fmt.Errorf("failed to parse identifier %q: %w", identifier, err)
	}
	
	// If already a UUID, return it
	if parsed.Type == domain.IdentifierTypeUUID {
		return parsed.UUID, nil
	}
	
	// Handle org-scoped resolution
	resourceOrgID := orgID
	if parsed.Type == domain.IdentifierTypeAbsoluteName {
		// Resolve org name to UUID first
		resolvedOrgID, err := r.ResolveOrganization(ctx, parsed.OrgName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve org %q for absolute name: %w", parsed.OrgName, err)
		}
		resourceOrgID = resolvedOrgID
	}
	
	// Query resource by name within org
	var result struct{ ID string }
	err = r.db.WithContext(ctx).
		Table(table).
		Select("id").
		Where("org_id = ?", resourceOrgID).
		Where("name = ?", parsed.Name).
		First(&result).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("%s not found in org %s: name=%q", table, resourceOrgID, parsed.Name)
		}
		return "", fmt.Errorf("database error querying %s: %w", table, err)
	}
	
	return result.ID, nil
}

