package domain

import (
	"fmt"
	"strings"
	"time"
)

// Unit represents a Terraform unit with its metadata
type Unit struct {
    ID       string    `json:"id"`
    Size     int64     `json:"size"`
    Updated  time.Time `json:"updated"`
    Locked   bool      `json:"locked"`
    LockInfo *Lock     `json:"lock,omitempty"`
}

// Lock represents lock information for a unit
type Lock struct {
    ID      string    `json:"id"`
    Who     string    `json:"who"`
    Version string    `json:"version"`
    Created time.Time `json:"created"`
}

// Version represents a version of a unit
type Version struct {
    Timestamp time.Time `json:"timestamp"`
    Hash      string    `json:"hash"`
    Size      int64     `json:"size"`
}

// ValidateUnitID validates a unit ID
func ValidateUnitID(id string) error {
    if id == "" {
        return fmt.Errorf("unit ID cannot be empty")
    }
    if strings.Contains(id, "..") {
        return fmt.Errorf("unit ID cannot contain '..'")
    }
    id = strings.Trim(id, "/")
    if id == "" {
        return fmt.Errorf("unit ID cannot be just slashes")
    }
    return nil
}

// NormalizeUnitID normalizes a unit ID
func NormalizeUnitID(id string) string {
    id = strings.Trim(id, "/")
    parts := strings.Split(id, "/")
    var cleanParts []string
    for _, part := range parts {
        if part != "" {
            cleanParts = append(cleanParts, part)
        }
    }
    return strings.Join(cleanParts, "/")
}

// FilterUnitsByPrefix filters units by prefix
func FilterUnitsByPrefix(units []*Unit, prefix string) []*Unit {
    if prefix == "" {
        return units
    }
    var filtered []*Unit
    for _, u := range units {
        if strings.HasPrefix(u.ID, prefix) {
            filtered = append(filtered, u)
        }
    }
    return filtered
}

// SortUnitsByID sorts units by ID alphabetically
func SortUnitsByID(units []*Unit) {
    n := len(units)
    for i := 0; i < n-1; i++ {
        for j := 0; j < n-i-1; j++ {
            if units[j].ID > units[j+1].ID {
                units[j], units[j+1] = units[j+1], units[j]
            }
        }
    }
}

// EncodeUnitID encodes a unit ID for use in URLs by replacing slashes with double underscores
func EncodeUnitID(id string) string { return strings.ReplaceAll(id, "/", "__") }

// DecodeUnitID decodes a URL-encoded unit ID by replacing double underscores with slashes
func DecodeUnitID(encoded string) string { return strings.ReplaceAll(encoded, "__", "/") }

