package utils

import (
	"fmt"
	"os"
	"strings"
)

func GetPublicBaseURL() string {
	return getBaseURL("PUBLIC_BASE_URL", "HOSTNAME", "https")
}

func GetInternalBaseURL() string {
	return getBaseURL("INTERNAL_BASE_URL", "HOSTNAME", "http")
}

func getBaseURL(primaryEnv string, fallbackEnv string, defaultScheme string) string {
	// Historically this codebase used HOSTNAME for both public URL generation
	// (for example GitHub App callback/webhook manifest URLs) and internal
	// service-to-self calls. We now prefer explicit PUBLIC_BASE_URL and
	// INTERNAL_BASE_URL, but keep HOSTNAME as a compatibility fallback so older
	// deployments and existing Helm secrets continue to work during migration.
	raw := strings.TrimSpace(os.Getenv(primaryEnv))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(fallbackEnv))
	}
	if raw == "" {
		return ""
	}

	raw = strings.TrimRight(raw, "/")
	if strings.Contains(raw, "://") {
		return raw
	}

	return fmt.Sprintf("%s://%s", defaultScheme, raw)
}
