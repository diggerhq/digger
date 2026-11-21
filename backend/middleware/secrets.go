package middleware

import "crypto/subtle"

// secretMatches reports whether presented equals the configured secret. It
// compares in constant time, and an empty configured secret matches nothing,
// so an unset environment variable can't authenticate an empty token.
func secretMatches(configured string, presented string) bool {
	if configured == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(configured), []byte(presented)) == 1
}
