package domain

import (
	"context"
	"fmt"
	"strings"
)

// OrgContext carries organization information through the request lifecycle
type OrgContext struct {
	OrgID string
}

// OrgService handles organization-related business logic
// This is pure domain logic with no dependencies on frameworks or infrastructure
type OrgService struct{}

// NewOrgService creates a new organization service
func NewOrgService() *OrgService {
	return &OrgService{}
}

// GetOrgPrefix returns the namespace prefix for an organization
// Format: org-<orgID>/
// This is the canonical org namespace format
func (s *OrgService) GetOrgPrefix(orgID string) string {
	return fmt.Sprintf("org-%s/", orgID)
}

// ValidateUnitBelongsToOrg checks if a unit ID belongs to the specified organization
// Returns an error if the unit does not have the correct org prefix
func (s *OrgService) ValidateUnitBelongsToOrg(unitID, orgID string) error {
	expectedPrefix := s.GetOrgPrefix(orgID)
	if !strings.HasPrefix(unitID, expectedPrefix) {
		return fmt.Errorf("unit '%s' does not belong to organization '%s'", unitID, orgID)
	}
	return nil
}

// GetOrgScopedPrefix returns the effective prefix for list operations
// Ensures that any user-provided prefix is scoped within the organization's namespace
func (s *OrgService) GetOrgScopedPrefix(orgID, userPrefix string) string {
	orgPrefix := s.GetOrgPrefix(orgID)
	
	if userPrefix == "" {
		// No user prefix, return org prefix
		return orgPrefix
	}
	
	// Ensure user prefix is within org namespace
	if !strings.HasPrefix(userPrefix, orgPrefix) {
		// User prefix doesn't include org prefix, prepend it
		return orgPrefix + userPrefix
	}
	
	// User prefix already includes org prefix
	return userPrefix
}

// IsOrgNamespaced checks if a unit ID is properly namespaced with an org prefix
func (s *OrgService) IsOrgNamespaced(unitID string) bool {
	return strings.HasPrefix(unitID, "org-") && strings.Contains(unitID, "/")
}

// ExtractOrgID extracts the organization ID from a namespaced unit ID
// Returns empty string if the unit is not properly namespaced
func (s *OrgService) ExtractOrgID(unitID string) string {
	if !s.IsOrgNamespaced(unitID) {
		return ""
	}
	
	// Format: org-<orgID>/...
	parts := strings.SplitN(unitID, "/", 2)
	if len(parts) < 1 {
		return ""
	}
	
	// Remove "org-" prefix
	return strings.TrimPrefix(parts[0], "org-")
}

// ============================================
// Context Management
// ============================================

// orgContextKey is used to store OrgContext in context.Context
type orgContextKey struct{}

// ContextWithOrg adds organization context to a context.Context
// This allows passing org information through the call stack without coupling to HTTP
func ContextWithOrg(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, orgContextKey{}, &OrgContext{OrgID: orgID})
}

// OrgFromContext retrieves organization context from context.Context
// Returns the OrgContext and a boolean indicating if it was found
func OrgFromContext(ctx context.Context) (*OrgContext, bool) {
	org, ok := ctx.Value(orgContextKey{}).(*OrgContext)
	return org, ok
}

