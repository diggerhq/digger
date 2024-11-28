package config

import (
	"os"
)

func LimitByNumOfFilesChanged() bool {
	// if this flag is set then it will fail if there are more projects impacted than the
	// number of files changed
	return os.Getenv("DIGGER_LIMIT_MAX_PROJECTS_TO_FILES_CHANGED") == "1"
}
