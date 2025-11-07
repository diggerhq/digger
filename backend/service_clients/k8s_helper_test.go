package service_clients

import (
	"testing"
)

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic alphanumeric
		{"SimpleAlphaNum", "abc123", "abc123"},
		// Remove illegal characters
		{"RemoveIllegalChars", "abc!@#123", "abc123"},
		// Trim leading/trailing dots/dashes/underscores
		{"TrimLeadingTrailing", "-._abc123-_.", "abc123"},
		// Keep valid internal characters
		{"KeepInternalSpecials", "ab-c_d.e", "ab-c_d.e"},
		// Collapse only removes invalid chars, not valid ones
		{"MixedValidInvalid", "a$bc.._123", "abc.._123"},
		// Entirely invalid string
		{"AllInvalid", "!@#$%^&*", ""},
		// Leading dot only
		{"LeadingDot", ".abc", "abc"},
		// Trailing underscore only
		{"TrailingUnderscore", "abc_", "abc"},
		// Consecutive invalids
		{"ConsecutiveInvalids", "a@@b##c", "abc"},
		// Empty input
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLabel(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeLabel(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
