package drift

import (
	"fmt"
	"os/exec"
	"strings"

	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
)

// GetLastChange returns the most recent commit that touched projectPath.
// The checkout must have full history (actions/checkout with fetch-depth: 0).
// Shallow clones are refused outright: their grafted tip commit diffs
// against the empty tree, so `git log -- .` would blame whoever made the
// repo's latest commit for every project. Errors are non-fatal to callers.
func GetLastChange(projectPath string) (*core_drift.LastChange, error) {
	shallowCmd := exec.Command("git", "rev-parse", "--is-shallow-repository")
	shallowCmd.Dir = projectPath
	shallowOut, err := shallowCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse failed for %v: %v", projectPath, err)
	}
	if strings.TrimSpace(string(shallowOut)) == "true" {
		return nil, fmt.Errorf("shallow clone detected for %v: last change attribution needs full git history (checkout with fetch-depth: 0)", projectPath)
	}
	// %x1f is the ASCII unit separator: unlike "|" it cannot appear in
	// author names, so fields always split cleanly.
	cmd := exec.Command("git", "log", "-1", "--format=%an%x1f%h%x1f%ar", "--", ".")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed for %v: %v", projectPath, err)
	}
	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\x1f", 3)
	if len(parts) < 3 || parts[0] == "" {
		return nil, fmt.Errorf("no git history found for %v", projectPath)
	}
	return &core_drift.LastChange{Author: parts[0], Commit: parts[1], When: parts[2]}, nil
}
