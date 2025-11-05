package domain

import "context"

// IdentifierResolver resolves human-readable identifiers (names, org-scoped names) 
// or UUIDs to their canonical UUID form.
// This is a domain interface - implementations live in the infrastructure layer.
type IdentifierResolver interface {
	// ResolveOrganization resolves an org identifier (name or UUID) to UUID
	ResolveOrganization(ctx context.Context, identifier string) (string, error)
	
	// GetOrganization retrieves full organization details by UUID
	// Used to populate context with org info to avoid repeated queries
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	
	// ResolveUnit resolves a unit identifier to UUID within an organization
	ResolveUnit(ctx context.Context, identifier, orgID string) (string, error)
	
	// ResolveRole resolves a role identifier to UUID within an organization
	ResolveRole(ctx context.Context, identifier, orgID string) (string, error)
	
	// ResolvePermission resolves a permission identifier to UUID within an organization
	ResolvePermission(ctx context.Context, identifier, orgID string) (string, error)
	
	// ResolveTag resolves a tag identifier to UUID within an organization
	ResolveTag(ctx context.Context, identifier, orgID string) (string, error)
}

