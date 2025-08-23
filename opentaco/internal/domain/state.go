package domain

import (
	"fmt"
	"strings"
	"time"
)

// State represents a Terraform state with its metadata
type State struct {
	ID       string    `json:"id"`
	Size     int64     `json:"size"`
	Updated  time.Time `json:"updated"`
	Locked   bool      `json:"locked"`
	LockInfo *Lock     `json:"lock,omitempty"`
}

// Lock represents lock information for a state
type Lock struct {
	ID      string    `json:"id"`
	Who     string    `json:"who"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

// ValidateStateID validates a state ID
func ValidateStateID(id string) error {
	if id == "" {
		return fmt.Errorf("state ID cannot be empty")
	}
	
	// Check for invalid characters
	if strings.Contains(id, "..") {
		return fmt.Errorf("state ID cannot contain '..'")
	}
	
	// Ensure no leading/trailing slashes
	id = strings.Trim(id, "/")
	if id == "" {
		return fmt.Errorf("state ID cannot be just slashes")
	}
	
	return nil
}

// NormalizeStateID normalizes a state ID
func NormalizeStateID(id string) string {
	// Trim leading/trailing slashes
	id = strings.Trim(id, "/")
	
	// Replace multiple slashes with single slash
	parts := strings.Split(id, "/")
	var cleanParts []string
	for _, part := range parts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}
	
	return strings.Join(cleanParts, "/")
}

// FilterStatesByPrefix filters states by prefix
func FilterStatesByPrefix(states []*State, prefix string) []*State {
	if prefix == "" {
		return states
	}
	
	var filtered []*State
	for _, state := range states {
		if strings.HasPrefix(state.ID, prefix) {
			filtered = append(filtered, state)
		}
	}
	
	return filtered
}

// SortStatesByID sorts states by ID alphabetically
func SortStatesByID(states []*State) {
	// Simple bubble sort for the scaffold
	n := len(states)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if states[j].ID > states[j+1].ID {
				states[j], states[j+1] = states[j+1], states[j]
			}
		}
	}
}

// EncodeStateID encodes a state ID for use in URLs by replacing slashes with double underscores
func EncodeStateID(id string) string {
	return strings.ReplaceAll(id, "/", "__")
}

// DecodeStateID decodes a URL-encoded state ID by replacing double underscores with slashes
func DecodeStateID(encoded string) string {
	return strings.ReplaceAll(encoded, "__", "/")
}