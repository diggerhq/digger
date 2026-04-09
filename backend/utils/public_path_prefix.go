package utils

import "strings"

// NormalizePublicPathPrefix normalizes a configured URL path prefix (e.g. "/orchestrator")
// so it can be safely concatenated with other paths.
//
// Examples:
// - "" or "/" -> ""
// - "orchestrator" -> "/orchestrator"
// - "/orchestrator/" -> "/orchestrator"
func NormalizePublicPathPrefix(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "/" {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return strings.TrimRight(raw, "/")
}

// ApplyPublicPathPrefix concatenates prefix + p ensuring p starts with "/".
func ApplyPublicPathPrefix(prefix, p string) string {
	if p == "" {
		return prefix
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return prefix + p
}
