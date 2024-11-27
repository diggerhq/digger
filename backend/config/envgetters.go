package config

import (
	"fmt"
	"os"
	"strconv"
)

func GetMaxProjectsCreated() int {
	// the maximum number of impacted projects possible for a change
	// digger will fail when this number exceeds it
	// default value of 0 or negative means unlimited allowed
	maxProjects := os.Getenv("DIGGER_MAX_PROJECTS_IMPACTED")
	maxProjectsNum, err := strconv.Atoi(maxProjects)
	if err != nil {
		fmt.Printf("Error converting env var to number: %v\n", err)
		return 0
	}
	return maxProjectsNum
}
