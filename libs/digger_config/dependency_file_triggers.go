package digger_config

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

var localTerraformModuleSourcePrefixes = []string{
	"./",
	"../",
	".\\",
	"..\\",
}

func enrichProjectsWithDependencyFileTriggers(config *DiggerConfig, repoRoot string) {
	absoluteRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		slog.Warn("failed to resolve repository root for dependency file triggers", "repoRoot", repoRoot, "error", err)
		return
	}

	for i := range config.Projects {
		project := &config.Projects[i]
		if !project.DependencyFileTriggers {
			continue
		}
		existingDependencyProjects := make(map[string]struct{}, len(project.DependencyProjects))
		for _, dependencyProject := range project.DependencyProjects {
			existingDependencyProjects[dependencyProject] = struct{}{}
		}

		patterns, err := getDependencyFileTriggerPatterns(absoluteRepoRoot, *project)
		if err != nil {
			slog.Warn("failed to discover dependency file triggers for project", "projectName", project.Name, "projectDir", project.Dir, "error", err)
			continue
		}
		if len(patterns) == 0 {
			continue
		}

		project.IncludePatterns = appendUniqueStrings(project.IncludePatterns, patterns...)
		inferredDependencyPatterns := inferDependencyProjectsFromPatterns(config.Projects, *project, patterns)
		for dependencyProjectName, dependencyPatterns := range inferredDependencyPatterns {
			project.DependencyProjects = appendUniqueStrings(project.DependencyProjects, dependencyProjectName)
			if _, hasManualDependency := existingDependencyProjects[dependencyProjectName]; hasManualDependency {
				continue
			}
			if project.InferredDependencyPatternsByProject == nil {
				project.InferredDependencyPatternsByProject = make(map[string][]string)
			}
			project.InferredDependencyPatternsByProject[dependencyProjectName] = appendUniqueStrings(
				project.InferredDependencyPatternsByProject[dependencyProjectName],
				dependencyPatterns...,
			)
		}
	}
}

func getDependencyFileTriggerPatterns(repoRoot string, project Project) ([]string, error) {
	switch {
	case project.Pulumi, project.Terragrunt:
		return nil, nil
	default:
		return getTerraformDependencyFileTriggerPatterns(repoRoot, project.Dir)
	}
}

func getTerraformDependencyFileTriggerPatterns(repoRoot string, projectDir string) ([]string, error) {
	absoluteProjectDir := filepath.Join(repoRoot, projectDir)
	modulePatterns, err := collectLocalTerraformModulePatterns(absoluteProjectDir, map[string]struct{}{})
	if err != nil {
		return nil, err
	}

	normalizedProjectDir := normalizeRelativePath(projectDir)
	patterns := make([]string, 0, len(modulePatterns))
	for _, modulePattern := range modulePatterns {
		relativePattern, err := filepath.Rel(repoRoot, modulePattern)
		if err != nil {
			return nil, err
		}

		normalizedPattern := normalizeRelativePath(relativePattern)
		if isOutsideRepo(normalizedPattern) {
			continue
		}

		if isPathInsideProject(normalizedProjectDir, normalizeRelativePath(filepath.Dir(normalizedPattern))) {
			continue
		}

		patterns = append(patterns, normalizedPattern)
	}

	sort.Strings(patterns)
	return appendUniqueStrings(nil, patterns...), nil
}

func collectLocalTerraformModulePatterns(moduleDir string, visited map[string]struct{}) ([]string, error) {
	absoluteModuleDir, err := filepath.Abs(moduleDir)
	if err != nil {
		return nil, err
	}

	if _, seen := visited[absoluteModuleDir]; seen {
		return nil, nil
	}
	visited[absoluteModuleDir] = struct{}{}

	if _, err := os.Stat(absoluteModuleDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	module, diags := tfconfig.LoadModule(absoluteModuleDir)
	if diags.HasErrors() {
		return nil, errors.New(diags.Error())
	}

	patterns := make([]string, 0, len(module.ModuleCalls))
	seenPatterns := make(map[string]struct{})

	for _, moduleCall := range module.ModuleCalls {
		if !isLocalTerraformModuleSource(moduleCall.Source) {
			continue
		}

		absoluteSourceDir := filepath.Clean(filepath.Join(absoluteModuleDir, moduleCall.Source))
		if _, err := os.Stat(absoluteSourceDir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				slog.Debug("skipping missing local terraform module source", "moduleDir", absoluteModuleDir, "source", moduleCall.Source)
				continue
			}
			return nil, err
		}

		modulePattern := filepath.Join(absoluteSourceDir, "*.tf*")
		if _, seen := seenPatterns[modulePattern]; !seen {
			patterns = append(patterns, modulePattern)
			seenPatterns[modulePattern] = struct{}{}
		}

		childPatterns, err := collectLocalTerraformModulePatterns(absoluteSourceDir, visited)
		if err != nil {
			return nil, err
		}

		for _, childPattern := range childPatterns {
			if _, seen := seenPatterns[childPattern]; seen {
				continue
			}
			patterns = append(patterns, childPattern)
			seenPatterns[childPattern] = struct{}{}
		}
	}

	sort.Strings(patterns)
	return patterns, nil
}

func isLocalTerraformModuleSource(raw string) bool {
	for _, prefix := range localTerraformModuleSourcePrefixes {
		if strings.HasPrefix(raw, prefix) {
			return true
		}
	}
	return false
}

func normalizeRelativePath(dir string) string {
	normalized := filepath.ToSlash(filepath.Clean(dir))
	if normalized == "" {
		return "."
	}
	return normalized
}

func isOutsideRepo(dir string) bool {
	return dir == ".." || strings.HasPrefix(dir, "../")
}

func isPathInsideProject(projectDir string, candidateDir string) bool {
	if projectDir == "." {
		return candidateDir == "."
	}
	return candidateDir == projectDir || strings.HasPrefix(candidateDir, projectDir+"/")
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing)+len(values))
	result := make([]string, 0, len(existing)+len(values))

	for _, value := range existing {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func inferDependencyProjectsFromPatterns(projects []Project, currentProject Project, patterns []string) map[string][]string {
	currentProjectDir := normalizeRelativePath(currentProject.Dir)
	dependencyProjectPatterns := make(map[string][]string)

	for _, pattern := range patterns {
		patternDir := normalizeRelativePath(filepath.Dir(pattern))
		if isOutsideRepo(patternDir) {
			continue
		}
		if isPathInsideProject(currentProjectDir, patternDir) {
			continue
		}

		mostSpecificProjectNames := findMostSpecificProjectNamesForDir(projects, currentProject.Name, patternDir)
		if len(mostSpecificProjectNames) == 0 {
			continue
		}

		for _, dependencyProject := range mostSpecificProjectNames {
			dependencyProjectPatterns[dependencyProject] = appendUniqueStrings(dependencyProjectPatterns[dependencyProject], pattern)
		}
	}

	return dependencyProjectPatterns
}

func findMostSpecificProjectNamesForDir(projects []Project, currentProjectName string, dependencyDir string) []string {
	maxSpecificity := -1
	projectNames := make([]string, 0)

	for _, candidate := range projects {
		if candidate.Name == currentProjectName {
			continue
		}

		candidateDir := normalizeRelativePath(candidate.Dir)
		if dependencyDir != candidateDir && !strings.HasPrefix(dependencyDir, candidateDir+"/") {
			continue
		}

		specificity := len(candidateDir)
		switch {
		case specificity > maxSpecificity:
			maxSpecificity = specificity
			projectNames = []string{candidate.Name}
		case specificity == maxSpecificity:
			projectNames = append(projectNames, candidate.Name)
		}
	}

	// Multiple projects can intentionally share a directory via workspaces, so
	// only infer a dependency when the upstream project is unambiguous.
	if len(projectNames) > 1 {
		return nil
	}

	sort.Strings(projectNames)
	return projectNames
}
