package validate

import (
	"regexp"
	"strings"
	"testing"
)

// Copy of the check logic (or import if public)
func IsValidPath(path string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_./:-]+$`, path)
	return matched && !strings.Contains(path, "..")
}

func FuzzIsValidPath(f *testing.F) {
	f.Add("/valid/path")
	f.Add("../escape")
	f.Add("./digger")
	f.Fuzz(func(t *testing.T, input string) {
		_ = IsValidPath(input) // ensure no panic
	})
}
