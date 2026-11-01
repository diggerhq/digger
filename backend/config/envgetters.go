package config

import (
	"github.com/spf13/cast"
	"log/slog"
	"os"
	"time"
)

func LimitByNumOfFilesChanged() bool {
	// if this flag is set then it will fail if there are more projects impacted than the
	// number of files changed
	return os.Getenv("DIGGER_LIMIT_MAX_PROJECTS_TO_FILES_CHANGED") == "1"
}

func MaxImpactedProjectsPerChange() int {
	m := os.Getenv("DIGGER_MAX_PROJECTS_PER_CHANGE")
	if m == "" {
		return 99999
	} else {
		v, err := cast.ToIntE(m)
		if err != nil {
			slog.Warn("unable to cast DIGGER_MAX_PROJECTS_PER_CHANGE to int, defaulting to 99999", "DIGGER_MAX_PROJECTS_PER_CHANGE", m)
			return 99999
		}
		return v
	}
}

// DuplicateCommandWindow is how far back to look for a running plan or apply
// of the same commit. Zero disables the check.
func DuplicateCommandWindow() time.Duration {
	const def = 2 * time.Hour // the job token's lifetime
	v := os.Getenv("DIGGER_DUPLICATE_COMMAND_WINDOW")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		slog.Warn("invalid DIGGER_DUPLICATE_COMMAND_WINDOW, using the default", "value", v, "default", def)
		return def
	}
	return d
}
