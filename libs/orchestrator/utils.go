package orchestrator

import (
	"errors"
	"regexp"
)

func ParseWorkspace(comment string) (string, error) {
	re := regexp.MustCompile(`-w(?:\s+(\S+)|$)`)
	matches := re.FindAllStringSubmatch(comment, -1)

	if len(matches) == 0 {
		return "", nil
	}

	if len(matches) > 1 {
		return "", errors.New("more than one -w flag found")
	}

	if len(matches[0]) < 2 || matches[0][1] == "" {
		return "", errors.New("no value found after -w flag")
	}

	return matches[0][1], nil
}

func ParseProjectName(comment string) string {
	re := regexp.MustCompile(`-p ([0-9a-zA-Z\-_]+)`)
	match := re.FindStringSubmatch(comment)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}
