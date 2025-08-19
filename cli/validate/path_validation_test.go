package validate

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// IsValidPath checks if a path is valid according to the shell script rules
func IsValidPath(path string) bool {
	// Check for invalid characters (only allow a-z, A-Z, 0-9, _, ., /, -)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_./-]+$`, path); !matched {
		return false
	}

	// Check for directory traversal
	if strings.Contains(path, "..") {
		return false
	}

	// Check for colons (not allowed)
	if strings.Contains(path, ":") {
		return false
	}

	// Must be an absolute path (starts with /)
	if !strings.HasPrefix(path, "/") {
		return false
	}

	return true
}

func FuzzIsValidPath(f *testing.F) {
	// Add seed corpus
	testCases := []string{
		"/valid/path",
		"../escape",
		"./digger",
		"/absolute/path",
		"/with-hyphen",
		"/with_underscore",
		"/with.dot",
		"/with/nested/path",
		"relative/path",
		"colon:in:path",
		"/path/with/../traversal",
		"/path/with/./dot",
		"/",
		"",
		" ",
		"*",
		"&",
		"|",
		";",
		"`",
		"'",
		"\"",
		"$",
		"(",
		")",
		"[",
		"]",
		"{",
		"}",
		"<",
		">",
		" ",
		"\t",
		"\n",
		"\r",
		"\x00",
	}

	for _, tc := range testCases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip("Skipping invalid UTF-8 string")
		}

		// Just execute the function to check for panics
		_ = IsValidPath(input)
	})
}
