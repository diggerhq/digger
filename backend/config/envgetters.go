package config

import (
	"github.com/spf13/cast"
	"log/slog"
	"os"
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
			slog.Warn("unable to cast %s to int, defaulting to 99999")
			return 99999
		}
		return v
	}
}
