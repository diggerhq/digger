package service_clients

import (
	"regexp"
	"strings"
)

// Allowed: (([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
// This means: optional string that starts and ends with alphanumeric,
// and may contain -, _, . in between.
// this is because k8s does not allow labels that do not match this patter
var allowedPattern = regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`)
var cleanupPattern = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)


func sanitizeLabel(input string) string {
	// Remove all disallowed characters
	cleaned := cleanupPattern.ReplaceAllString(input, "")
	// Trim leading and trailing non-alphanumerics
	cleaned = strings.Trim(cleaned, "-_.")
	return cleaned
}