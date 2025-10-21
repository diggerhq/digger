package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"gorm.io/gorm"
)

// Identifier parsing types and functions
var (
	ErrInvalidIdentifier   = errors.New("invalid identifier format")
	ErrAmbiguousIdentifier = errors.New("identifier matches multiple resources")
	
	uuidPattern         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	absoluteNamePattern = regexp.MustCompile(`^org-([a-zA-Z0-9_-]+)/(.+)$`)
)

type IdentifierType int

const (
	IdentifierTypeUUID IdentifierType = iota
	IdentifierTypeName
	IdentifierTypeAbsoluteName
)

type ParsedIdentifier struct {
	Type    IdentifierType
	UUID    string
	Name    string
	OrgName string
}

// ParseIdentifier parses an identifier string into its components.
// Supports three formats:
//   - UUID: "a1b2c3d4-1234-5678-90ab-cdef12345678"
//   - Simple name: "dev" (resolved within current org context)
//   - Absolute name: "org-acme/dev" (explicitly specifies org)
func ParseIdentifier(identifier string) (*ParsedIdentifier, error) {
	if identifier == "" {
		return nil, fmt.Errorf("%w: empty identifier", ErrInvalidIdentifier)
	}
	
	if IsUUID(identifier) {
		return &ParsedIdentifier{
			Type: IdentifierTypeUUID,
			UUID: identifier,
		}, nil
	}
	
	if matches := absoluteNamePattern.FindStringSubmatch(identifier); matches != nil {
		return &ParsedIdentifier{
			Type:    IdentifierTypeAbsoluteName,
			OrgName: matches[1],
			Name:    matches[2],
		}, nil
	}
	
	return &ParsedIdentifier{
		Type: IdentifierTypeName,
		Name: identifier,
	}, nil
}

// BuildAbsoluteName constructs an org-scoped absolute name
func BuildAbsoluteName(orgName, resourceName string) string {
	return fmt.Sprintf("org-%s/%s", orgName, resourceName)
}

// IsUUID checks if a string is a valid UUID
func IsUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

// ResourceResolver resolves identifiers (UUID, name, or org-scoped name) to UUIDs for all resource types
type ResourceResolver struct {
	db *gorm.DB
}

func NewResourceResolver(db interface{}) *ResourceResolver {
	gormDB, ok := db.(*gorm.DB)
	if !ok {
		return &ResourceResolver{}
	}
	return &ResourceResolver{db: gormDB}
}

// ResolveUnit resolves unit identifier to UUID
func (r *ResourceResolver) ResolveUnit(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "units", identifier, orgID)
}

// ResolveRole resolves role identifier to UUID
func (r *ResourceResolver) ResolveRole(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "roles", identifier, orgID)
}

// ResolvePermission resolves permission identifier to UUID
func (r *ResourceResolver) ResolvePermission(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "permissions", identifier, orgID)
}

// ResolveTag resolves tag identifier to UUID
func (r *ResourceResolver) ResolveTag(ctx context.Context, identifier, orgID string) (string, error) {
	return r.resolveResource(ctx, "tags", identifier, orgID)
}

// ResolveOrganization resolves organization identifier to UUID
func (r *ResourceResolver) ResolveOrganization(ctx context.Context, identifier string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database not available")
	}
	
	parsed, err := ParseIdentifier(identifier)
	if err != nil {
		return "", err
	}
	
	if parsed.Type == IdentifierTypeUUID {
		return parsed.UUID, nil
	}
	
	var result struct{ ID string }
	err = r.db.WithContext(ctx).
		Table("organizations").
		Select("id").
		Where("org_id = ?", parsed.Name).
		First(&result).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("organization not found: %s", parsed.Name)
		}
		return "", err
	}
	
	return result.ID, nil
}

func (r *ResourceResolver) resolveResource(ctx context.Context, table, identifier, orgID string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database not available")
	}
	
	parsed, err := ParseIdentifier(identifier)
	if err != nil {
		return "", err
	}
	
	if parsed.Type == IdentifierTypeUUID {
		return parsed.UUID, nil
	}
	
	resourceOrgID := orgID
	if parsed.Type == IdentifierTypeAbsoluteName {
		resourceOrgID = parsed.OrgName
	}
	
	var result struct{ ID string }
	err = r.db.WithContext(ctx).
		Table(table).
		Select(table + ".id").
		Joins("JOIN organizations ON organizations.id = " + table + ".org_id").
		Where("organizations.org_id = ?", resourceOrgID).
		Where(table + ".name = ?", parsed.Name).
		First(&result).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("%s not found: %s", table, parsed.Name)
		}
		return "", err
	}
	
	return result.ID, nil
}

