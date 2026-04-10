package tac

import (
	"errors"
	"os"
	"path/filepath"
)

func InferProjectWhenModifiedPatterns(gitRoot string, projectDir string, ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, cascadeDependencies bool, triggerProjectsFromDirOnly bool) ([]string, error) {
	sourcePath := filepath.Join(gitRoot, projectDir, "terragrunt.hcl")
	if _, err := os.Stat(sourcePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	state := newDependencyDiscoveryState()
	project, _, err := createProject(state, ignoreParentTerragrunt, ignoreDependencyBlocks, gitRoot, cascadeDependencies, "default", []string{}, true, "", false, false, sourcePath, triggerProjectsFromDirOnly, "_")
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, nil
	}

	return project.Autoplan.WhenModified, nil
}
