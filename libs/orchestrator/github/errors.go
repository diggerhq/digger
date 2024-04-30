package github

import (
	"errors"
)

var UnhandledMergeGroupEventError = errors.New("ignoring event: merge_group")
