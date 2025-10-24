package domain

import "context"

// IdentifierResolver resolves identifiers to UUIDs
type IdentifierResolver interface {
	// ResolveOrganization accepts UUID or external ID (format: "ext:provider:id")
	// Does NOT accept names (names are not unique)
	ResolveOrganization(ctx context.Context, identifier string) (string, error)
	
	// ResolveUnit accepts UUID, name (org-scoped), or absolute name
	ResolveUnit(ctx context.Context, identifier, orgID string) (string, error)
	
	ResolveRole(ctx context.Context, identifier, orgID string) (string, error)
	ResolvePermission(ctx context.Context, identifier, orgID string) (string, error)
	ResolveTag(ctx context.Context, identifier, orgID string) (string, error)
}

