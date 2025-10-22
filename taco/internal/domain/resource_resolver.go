package domain

import (
	"errors"
	"fmt"
	"regexp"
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

// Note: The ResourceResolver implementation has been moved to 
// internal/repositories/identifier_resolver.go for clean architecture.
// Use domain.IdentifierResolver interface instead.
