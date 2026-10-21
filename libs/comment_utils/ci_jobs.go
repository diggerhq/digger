package comment_utils

import (
	"fmt"
	"os"

	"github.com/diggerhq/digger/libs/ci/generic"
)

func GetWorkflowUrl() string {
	runId := os.Getenv("GITHUB_RUN_ID")
	if generic.IsForgejoActions() {
		runId = os.Getenv("GITHUB_RUN_NUMBER")
	} else if os.Getenv("GITHUB_ACTIONS") != "true" {
		return "#"
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s", os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), runId)
}
