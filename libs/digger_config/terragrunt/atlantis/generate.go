package atlantis

import (
	"context"
	"github.com/gruntwork-io/terragrunt/cli/commands/terraform"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"regexp"
	"sort"

	"github.com/hashicorp/go-getter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Parse env vars into a map
func getEnvs() map[string]string {
	envs := os.Environ()
	m := make(map[string]string)

	for _, env := range envs {
		results := strings.Split(env, "=")
		m[results[0]] = results[1]
	}

	return m
}

// Terragrunt imports can be relative or absolute
// This makes relative paths absolute
func makePathAbsolute(gitRoot string, path string, parentPath string) string {
	if strings.HasPrefix(path, filepath.ToSlash(gitRoot)) {
		return path
	}

	parentDir := filepath.Dir(parentPath)
	return filepath.Join(parentDir, path)
}

var requestGroup singleflight.Group

// Set up a cache for the getDependencies function
type getDependenciesOutput struct {
	dependencies []string
	err          error
}

type GetDependenciesCache struct {
	mtx  sync.RWMutex
	data map[string]getDependenciesOutput
}

func newGetDependenciesCache() *GetDependenciesCache {
	return &GetDependenciesCache{data: map[string]getDependenciesOutput{}}
}

func (m *GetDependenciesCache) set(k string, v getDependenciesOutput) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.data[k] = v
}

func (m *GetDependenciesCache) get(k string) (getDependenciesOutput, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	v, ok := m.data[k]
	return v, ok
}

var getDependenciesCache = newGetDependenciesCache()

func uniqueStrings(str []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range str {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func lookupProjectHcl(m map[string][]string, value string) (key string) {
	for k, values := range m {
		for _, val := range values {
			if val == value {
				key = k
				return
			}
		}
	}
	return key
}

// sliceUnion takes two slices of strings and produces a union of them, containing only unique values
func sliceUnion(a, b []string) []string {
	m := make(map[string]bool)

	for _, item := range a {
		m[item] = true
	}

	for _, item := range b {
		if _, ok := m[item]; !ok {
			a = append(a, item)
		}
	}
	return a
}

// Parses the terragrunt digger_config at `path` to find all modules it depends on
func getDependencies(ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, gitRoot string, cascadeDependencies bool, path string, terragruntOptions *options.TerragruntOptions) ([]string, bool, error) {
	res, err, _ := requestGroup.Do(path, func() (interface{}, error) {
		// Check if this path has already been computed
		cachedResult, ok := getDependenciesCache.get(path)
		if ok {
			return cachedResult.dependencies, cachedResult.err
		}

		// parse the module path to find what it includes, as well as its potential to be a parent
		// return nils to indicate we should skip this project
		isParent, includes, err := parseModule(path, terragruntOptions)
		if err != nil {
			getDependenciesCache.set(path, getDependenciesOutput{nil, err})
			return nil, err
		}
		if isParent && ignoreParentTerragrunt {
			getDependenciesCache.set(path, getDependenciesOutput{nil, nil})
			return nil, nil
		}

		dependencies := []string{}
		if len(includes) > 0 {
			for _, includeDep := range includes {
				getDependenciesCache.set(includeDep.Path, getDependenciesOutput{nil, err})
				dependencies = append(dependencies, includeDep.Path)
			}
		}

		// Parse the HCL file
		decodeTypes := []config.PartialDecodeSectionType{
			config.DependencyBlock,
			config.DependenciesBlock,
			config.TerraformBlock,
		}
		parsedConfig, err := config.PartialParseConfigFile(path, terragruntOptions, nil, decodeTypes)
		if err != nil {
			getDependenciesCache.set(path, getDependenciesOutput{nil, err})
			return nil, err
		}

		// Parse out locals
		locals, err := parseLocals(path, terragruntOptions, nil)
		if err != nil {
			getDependenciesCache.set(path, getDependenciesOutput{nil, err})
			return nil, err
		}

		// Get deps from locals
		if locals.ExtraAtlantisDependencies != nil {
			dependencies = sliceUnion(dependencies, locals.ExtraAtlantisDependencies)
		}

		// Get deps from `dependencies` and `dependency` blocks
		if parsedConfig.Dependencies != nil && !ignoreDependencyBlocks {
			for _, parsedPaths := range parsedConfig.Dependencies.Paths {
				dependencies = append(dependencies, filepath.Join(parsedPaths, "terragrunt.hcl"))
			}
		}

		// Get deps from the `Source` field of the `Terraform` block
		if parsedConfig.Terraform != nil && parsedConfig.Terraform.Source != nil {
			source := parsedConfig.Terraform.Source

			// Use `go-getter` to normalize the source paths
			parsedSource, err := getter.Detect(*source, filepath.Dir(path), getter.Detectors)
			if err != nil {
				return nil, err
			}

			// Check if the path begins with a drive letter, denoting Windows
			isWindowsPath, err := regexp.MatchString(`^[A-Z]:`, parsedSource)
			if err != nil {
				return nil, err
			}

			// If the normalized source begins with `file://`, or matched the Windows drive letter check, it is a local path
			if strings.HasPrefix(parsedSource, "file://") || isWindowsPath {
				// Remove the prefix so we have a valid filesystem path
				parsedSource = strings.TrimPrefix(parsedSource, "file://")

				dependencies = append(dependencies, filepath.Join(parsedSource, "*.tf*"))

				ls, err := parseTerraformLocalModuleSource(parsedSource)
				if err != nil {
					return nil, err
				}
				sort.Strings(ls)

				dependencies = append(dependencies, ls...)
			}
		}

		// Get deps from `extra_arguments` fields of the `Terraform` block
		if parsedConfig.Terraform != nil && parsedConfig.Terraform.ExtraArgs != nil {
			extraArgs := parsedConfig.Terraform.ExtraArgs
			for _, arg := range extraArgs {
				if arg.RequiredVarFiles != nil {
					dependencies = append(dependencies, *arg.RequiredVarFiles...)
				}
				if arg.OptionalVarFiles != nil {
					dependencies = append(dependencies, *arg.OptionalVarFiles...)
				}
				if arg.Arguments != nil {
					for _, cliFlag := range *arg.Arguments {
						if strings.HasPrefix(cliFlag, "-var-file=") {
							dependencies = append(dependencies, strings.TrimPrefix(cliFlag, "-var-file="))
						}
					}
				}
			}
		}

		// Filter out and dependencies that are the empty string
		nonEmptyDeps := []string{}
		for _, dep := range dependencies {
			if dep != "" {
				childDepAbsPath := dep
				if !filepath.IsAbs(childDepAbsPath) {
					childDepAbsPath = makePathAbsolute(gitRoot, dep, path)
				}
				childDepAbsPath = filepath.ToSlash(childDepAbsPath)
				nonEmptyDeps = append(nonEmptyDeps, childDepAbsPath)
			}
		}

		// Recurse to find dependencies of all dependencies
		cascadedDeps := []string{}
		for _, dep := range nonEmptyDeps {
			cascadedDeps = append(cascadedDeps, dep)

			// The "cascading" feature is protected by a flag
			if !cascadeDependencies {
				continue
			}

			depPath := dep
			terrOpts, _ := options.NewTerragruntOptionsWithConfigPath(depPath)
			terrOpts.OriginalTerragruntConfigPath = terragruntOptions.OriginalTerragruntConfigPath
			childDeps, skipProject, err := getDependencies(ignoreParentTerragrunt, ignoreDependencyBlocks, gitRoot, cascadeDependencies, depPath, terrOpts)
			if err != nil {
				continue
			}
			if skipProject {
				continue
			}

			for _, childDep := range childDeps {
				// If `childDep` is a relative path, it will be relative to `childDep`, as it is from the nested
				// `getDependencies` call on the top level module's dependencies. So here we update any relative
				// path to be from the top level module instead.
				childDepAbsPath := childDep
				if !filepath.IsAbs(childDep) {
					childDepAbsPath, err = filepath.Abs(filepath.Join(depPath, "..", childDep))
					if err != nil {
						getDependenciesCache.set(path, getDependenciesOutput{nil, err})
						return nil, err
					}
				}
				childDepAbsPath = filepath.ToSlash(childDepAbsPath)

				// Ensure we are not adding a duplicate dependency
				alreadyExists := false
				for _, dep := range cascadedDeps {
					if dep == childDepAbsPath {
						alreadyExists = true
						break
					}
				}
				if !alreadyExists {
					cascadedDeps = append(cascadedDeps, childDepAbsPath)
				}
			}
		}

		if filepath.Base(path) == "terragrunt.hcl" {
			dir := filepath.Dir(path)

			ls, err := parseTerraformLocalModuleSource(dir)
			if err != nil {
				return nil, err
			}
			sort.Strings(ls)

			cascadedDeps = append(cascadedDeps, ls...)
		}

		getDependenciesCache.set(path, getDependenciesOutput{cascadedDeps, err})
		return cascadedDeps, nil
	})

	if res != nil {
		return res.([]string), false, err
	} else {
		return nil, true, err
	}
}

// Creates an AtlantisProject for a directory
func createProject(ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, gitRoot string, cascadeDependencies bool, defaultWorkflow string, defaultApplyRequirements []string, autoPlan bool, defaultTerraformVersion string, createProjectName bool, createWorkspace bool, sourcePath string) (*AtlantisProject, []string, error) {
	options, err := options.NewTerragruntOptionsWithConfigPath(sourcePath)

	var potentialProjectDependencies []string
	if err != nil {
		return nil, potentialProjectDependencies, err
	}
	options.OriginalTerragruntConfigPath = sourcePath
	options.RunTerragrunt = terraform.Run
	options.Env = getEnvs()

	dependencies, skipProject, err := getDependencies(ignoreParentTerragrunt, ignoreDependencyBlocks, gitRoot, cascadeDependencies, sourcePath, options)
	if err != nil {
		return nil, potentialProjectDependencies, err
	}

	// dependencies being nil is a sign from `getDependencies` that this project should be skipped
	if skipProject == true {
		return nil, potentialProjectDependencies, nil
	}

	absoluteSourceDir := filepath.Dir(sourcePath) + string(filepath.Separator)

	locals, err := parseLocals(sourcePath, options, nil)
	if err != nil {
		return nil, potentialProjectDependencies, err
	}

	// If `atlantis_skip` is true on the module, then do not produce a project for it
	if locals.Skip != nil && *locals.Skip {
		return nil, potentialProjectDependencies, nil
	}

	// All dependencies depend on their own .hcl file, and any tf files in their directory
	relativeDependencies := []string{
		"*.hcl",
		"*.tf*",
	}

	// Add other dependencies based on their relative paths. We always want to output with Unix path separators
	for _, dependencyPath := range dependencies {
		absolutePath := dependencyPath
		if !filepath.IsAbs(absolutePath) {
			absolutePath = makePathAbsolute(gitRoot, dependencyPath, sourcePath)
		}
		potentialProjectDependencies = append(potentialProjectDependencies, projectNameFromDir(filepath.Dir(strings.TrimPrefix(absolutePath, gitRoot))))

		relativePath, err := filepath.Rel(absoluteSourceDir, absolutePath)
		if err != nil {
			return nil, potentialProjectDependencies, err
		}

		relativeDependencies = append(relativeDependencies, filepath.ToSlash(relativePath))
	}

	// Clean up the relative path to the format Atlantis expects
	relativeSourceDir := strings.TrimPrefix(absoluteSourceDir, gitRoot)
	relativeSourceDir = strings.TrimSuffix(relativeSourceDir, string(filepath.Separator))
	if relativeSourceDir == "" {
		relativeSourceDir = "."
	}

	workflow := defaultWorkflow
	if locals.AtlantisWorkflow != "" {
		workflow = locals.AtlantisWorkflow
	}

	applyRequirements := &defaultApplyRequirements
	if len(defaultApplyRequirements) == 0 {
		applyRequirements = nil
	}
	if locals.ApplyRequirements != nil {
		applyRequirements = &locals.ApplyRequirements
	}

	resolvedAutoPlan := autoPlan
	if locals.AutoPlan != nil {
		resolvedAutoPlan = *locals.AutoPlan
	}

	terraformVersion := defaultTerraformVersion
	if locals.TerraformVersion != "" {
		terraformVersion = locals.TerraformVersion
	}

	project := &AtlantisProject{
		Dir:               filepath.ToSlash(relativeSourceDir),
		Workflow:          workflow,
		TerraformVersion:  terraformVersion,
		ApplyRequirements: applyRequirements,
		Autoplan: AutoplanConfig{
			Enabled:      resolvedAutoPlan,
			WhenModified: uniqueStrings(relativeDependencies),
		},
	}

	projectName := projectNameFromDir(project.Dir)

	if createProjectName {
		project.Name = projectName
	}

	if createWorkspace {
		project.Workspace = projectName
	}

	return project, potentialProjectDependencies, nil
}

func projectNameFromDir(projectDir string) string {
	// Terraform Cloud limits the workspace names to be less than 90 characters
	// with letters, numbers, -, and _
	// https://www.terraform.io/docs/cloud/workspaces/naming.html
	// It is not clear from documentation whether the normal workspaces have those limitations
	// However a workspace 97 chars long has been working perfectly.
	// We are going to use the same name for both workspace & project name as it is unique.
	regex := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	projectName := regex.ReplaceAllString(projectDir, "_")
	return projectName
}

func createHclProject(defaultWorkflow string, defaultApplyRequirements []string, autoplan bool, useProjectMarkers bool, defaultTerraformVersion string, ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, gitRoot string, cascadeDependencies bool, createProjectName bool, createWorkspace bool, sourcePaths []string, workingDir string, projectHcl string) (*AtlantisProject, error) {
	var projectHclDependencies []string
	var childDependencies []string
	workflow := defaultWorkflow
	applyRequirements := &defaultApplyRequirements
	resolvedAutoPlan := autoplan
	terraformVersion := defaultTerraformVersion

	projectHclFile := filepath.Join(workingDir, projectHcl)
	projectHclOptions, err := options.NewTerragruntOptionsWithConfigPath(workingDir)
	if err != nil {
		return nil, err
	}
	projectHclOptions.RunTerragrunt = terraform.Run
	projectHclOptions.Env = getEnvs()

	locals, err := parseLocals(projectHclFile, projectHclOptions, nil)
	if err != nil {
		return nil, err
	}

	// If `atlantis_skip` is true on the module, then do not produce a project for it
	if locals.Skip != nil && *locals.Skip {
		return nil, nil
	}

	// if project markers are enabled, check if locals are set
	markedProject := false
	if locals.markedProject != nil {
		markedProject = *locals.markedProject
	}
	if useProjectMarkers && !markedProject {
		return nil, nil
	}

	if locals.ExtraAtlantisDependencies != nil {
		for _, dep := range locals.ExtraAtlantisDependencies {
			relDep, err := filepath.Rel(workingDir, dep)
			if err != nil {
				return nil, err
			}
			projectHclDependencies = append(projectHclDependencies, filepath.ToSlash(relDep))
		}
	}

	if locals.AtlantisWorkflow != "" {
		workflow = locals.AtlantisWorkflow
	}

	if len(defaultApplyRequirements) == 0 {
		applyRequirements = nil
	}
	if locals.ApplyRequirements != nil {
		applyRequirements = &locals.ApplyRequirements
	}

	if locals.AutoPlan != nil {
		resolvedAutoPlan = *locals.AutoPlan
	}

	if locals.TerraformVersion != "" {
		terraformVersion = locals.TerraformVersion
	}

	// build dependencies for terragrunt childs in directories below project hcl file
	for _, sourcePath := range sourcePaths {
		options, err := options.NewTerragruntOptionsWithConfigPath(sourcePath)
		if err != nil {
			return nil, err
		}
		options.RunTerragrunt = terraform.Run
		options.Env = getEnvs()

		dependencies, skipProject, err := getDependencies(ignoreParentTerragrunt, ignoreDependencyBlocks, gitRoot, cascadeDependencies, sourcePath, options)
		if err != nil {
			return nil, err
		}
		// dependencies being nil is a sign from `getDependencies` that this project should be skipped
		if skipProject == true {
			return nil, nil
		}

		// All dependencies depend on their own .hcl file, and any tf files in their directory
		relativeDependencies := []string{
			"*.hcl",
			"*.tf*",
			"**/*.hcl",
			"**/*.tf*",
		}

		// Add other dependencies based on their relative paths. We always want to output with Unix path separators
		for _, dependencyPath := range dependencies {
			absolutePath := dependencyPath
			if !filepath.IsAbs(absolutePath) {
				absolutePath = makePathAbsolute(gitRoot, dependencyPath, sourcePath)
			}

			relativePath, err := filepath.Rel(workingDir, absolutePath)
			if err != nil {
				return nil, err
			}

			if !strings.Contains(absolutePath, filepath.ToSlash(workingDir)) {
				relativeDependencies = append(relativeDependencies, filepath.ToSlash(relativePath))
			}
		}

		childDependencies = append(childDependencies, relativeDependencies...)
	}
	dir, err := filepath.Rel(gitRoot, workingDir)
	if err != nil {
		return nil, err
	}

	project := &AtlantisProject{
		Dir:               filepath.ToSlash(dir),
		Workflow:          workflow,
		TerraformVersion:  terraformVersion,
		ApplyRequirements: applyRequirements,
		Autoplan: AutoplanConfig{
			Enabled:      resolvedAutoPlan,
			WhenModified: uniqueStrings(append(childDependencies, projectHclDependencies...)),
		},
	}

	// Terraform Cloud limits the workspace names to be less than 90 characters
	// with letters, numbers, -, and _
	// https://www.terraform.io/docs/cloud/workspaces/naming.html
	// It is not clear from documentation whether the normal workspaces have those limitations
	// However a workspace 97 chars long has been working perfectly.
	// We are going to use the same name for both workspace & project name as it is unique.
	regex := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	projectName := regex.ReplaceAllString(project.Dir, "_")

	if createProjectName {
		project.Name = projectName
	}

	if createWorkspace {
		project.Workspace = projectName
	}

	return project, nil
}

// Finds the absolute paths of all terragrunt.hcl files
func getAllTerragruntFiles(filterPath string, projectHclFiles []string, path string) ([]string, error) {
	options, err := options.NewTerragruntOptionsWithConfigPath(path)
	if err != nil {
		return nil, err
	}

	// If filterPath is provided, override workingPath instead of gitRoot
	// We do this here because we want to keep the relative path structure of Terragrunt files
	// to root and just ignore the ConfigFiles
	workingPaths := []string{path}

	// filters are not working (yet) if using project hcl files (which are kind of filters by themselves)
	if filterPath != "" && len(projectHclFiles) == 0 {
		// get all matching folders
		workingPaths, err = filepath.Glob(filterPath)
		if err != nil {
			return nil, err
		}
	}

	uniqueConfigFilePaths := make(map[string]bool)
	orderedConfigFilePaths := []string{}
	for _, workingPath := range workingPaths {
		paths, err := config.FindConfigFilesInPath(workingPath, options)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			// if path not yet seen, insert once
			if !uniqueConfigFilePaths[p] {
				orderedConfigFilePaths = append(orderedConfigFilePaths, p)
				uniqueConfigFilePaths[p] = true
			}
		}
	}

	uniqueConfigFileAbsPaths := []string{}
	for _, uniquePath := range orderedConfigFilePaths {
		uniqueAbsPath, err := filepath.Abs(uniquePath)
		if err != nil {
			return nil, err
		}
		uniqueConfigFileAbsPaths = append(uniqueConfigFileAbsPaths, uniqueAbsPath)
	}

	return uniqueConfigFileAbsPaths, nil
}

// Finds the absolute paths of all arbitrary project hcl files
func getAllTerragruntProjectHclFiles(projectHclFiles []string, gitRoot string) map[string][]string {
	orderedHclFilePaths := map[string][]string{}
	uniqueHclFileAbsPaths := map[string][]string{}
	for _, projectHclFile := range projectHclFiles {
		err := filepath.Walk(gitRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && info.Name() == projectHclFile {
				orderedHclFilePaths[projectHclFile] = append(orderedHclFilePaths[projectHclFile], filepath.Dir(path))
			}

			return nil
		})

		if err != nil {
			log.Fatal(err)
		}

		for _, uniquePath := range orderedHclFilePaths[projectHclFile] {
			uniqueAbsPath, err := filepath.Abs(uniquePath)
			if err != nil {
				return nil
			}
			uniqueHclFileAbsPaths[projectHclFile] = append(uniqueHclFileAbsPaths[projectHclFile], uniqueAbsPath)
		}
	}
	return uniqueHclFileAbsPaths
}

func Parse(gitRoot string, projectHclFiles []string, createHclProjectExternalChilds bool, autoMerge bool, parallel bool, filterPath string, createHclProjectChilds bool, ignoreParentTerragrunt bool, ignoreDependencyBlocks bool, cascadeDependencies bool, defaultWorkflow string, defaultApplyRequirements []string, autoPlan bool, defaultTerraformVersion string, createProjectName bool, createWorkspace bool, preserveProjects bool, useProjectMarkers bool) (*AtlantisConfig, map[string][]string, error) {
	// Ensure the gitRoot has a trailing slash and is an absolute path
	absoluteGitRoot, err := filepath.Abs(gitRoot)
	if err != nil {
		return nil, nil, err
	}
	gitRoot = absoluteGitRoot + string(filepath.Separator)
	workingDirs := []string{gitRoot}
	projectHclDirMap := map[string][]string{}
	var projectHclDirs []string
	if len(projectHclFiles) > 0 {
		workingDirs = nil
		// map [project-hcl-file] => directories containing project-hcl-file
		projectHclDirMap = getAllTerragruntProjectHclFiles(projectHclFiles, gitRoot)
		for _, projectHclFile := range projectHclFiles {
			projectHclDirs = append(projectHclDirs, projectHclDirMap[projectHclFile]...)
			workingDirs = append(workingDirs, projectHclDirMap[projectHclFile]...)
		}
		// parse terragrunt child modules outside the scope of projectHclDirs
		if createHclProjectExternalChilds {
			workingDirs = append(workingDirs, gitRoot)
		}
	}
	atlantisConfig := AtlantisConfig{
		Version:       3,
		AutoMerge:     autoMerge,
		ParallelPlan:  parallel,
		ParallelApply: parallel,
	}

	lock := sync.Mutex{}
	ctx := context.Background()
	errGroup, _ := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(10)
	potentialProjectDependencies := make(map[string][]string)
	for _, workingDir := range workingDirs {
		terragruntFiles, err := getAllTerragruntFiles(filterPath, projectHclFiles, workingDir)
		if err != nil {
			return nil, nil, err
		}

		if len(projectHclDirs) == 0 || createHclProjectChilds || (createHclProjectExternalChilds && workingDir == gitRoot) {
			// Concurrently looking all dependencies
			for _, terragruntPath := range terragruntFiles {
				terragruntPath := terragruntPath // https://golang.org/doc/faq#closures_and_goroutines

				// don't create atlantis projects already covered by project hcl file projects
				skipProject := false
				if createHclProjectExternalChilds && workingDir == gitRoot && len(projectHclDirs) > 0 {
					for _, projectHclDir := range projectHclDirs {
						if strings.HasPrefix(terragruntPath, projectHclDir) {
							skipProject = true
							break
						}
					}
				}
				if skipProject {
					continue
				}
				err := sem.Acquire(ctx, 1)
				if err != nil {
					return nil, nil, err
				}

				errGroup.Go(func() error {
					defer sem.Release(1)
					project, projDeps, err := createProject(ignoreParentTerragrunt, ignoreDependencyBlocks, gitRoot, cascadeDependencies, defaultWorkflow, defaultApplyRequirements, autoPlan, defaultTerraformVersion, createProjectName, createWorkspace, terragruntPath)
					if err != nil {
						return err
					}
					// if project and err are nil then skip this project
					if err == nil && project == nil {
						return nil
					}

					potentialProjectDependencies[project.Name] = projDeps

					// Lock the list as only one goroutine should be writing to atlantisConfig.Projects at a time
					lock.Lock()
					defer lock.Unlock()

					// When preserving existing projects, we should update existing blocks instead of creating a
					// duplicate, when generating something which already has representation
					if preserveProjects {
						updateProject := false

						// TODO: with Go 1.19, we can replace for loop with slices.IndexFunc for increased performance
						for i := range atlantisConfig.Projects {
							if atlantisConfig.Projects[i].Dir == project.Dir {
								updateProject = true
								log.Info("Updated project for ", terragruntPath)
								atlantisConfig.Projects[i] = *project

								// projects should be unique, let's exit for loop for performance
								// once first occurrence is found and replaced
								break
							}
						}

						if !updateProject {
							log.Info("Created project for ", terragruntPath)
							atlantisConfig.Projects = append(atlantisConfig.Projects, *project)
						}
					} else {
						log.Info("Created project for ", terragruntPath)
						atlantisConfig.Projects = append(atlantisConfig.Projects, *project)
					}

					return nil
				})
			}

			if err := errGroup.Wait(); err != nil {
				return nil, nil, err
			}
		}
		if len(projectHclDirs) > 0 && workingDir != gitRoot {
			projectHcl := lookupProjectHcl(projectHclDirMap, workingDir)
			err := sem.Acquire(ctx, 1)
			if err != nil {
				return nil, nil, err
			}

			errGroup.Go(func() error {
				defer sem.Release(1)
				project, err := createHclProject(defaultWorkflow, defaultApplyRequirements, autoPlan, useProjectMarkers, defaultTerraformVersion, ignoreParentTerragrunt, ignoreDependencyBlocks, gitRoot, cascadeDependencies, createProjectName, createWorkspace, terragruntFiles, workingDir, projectHcl)
				if err != nil {
					return err
				}
				// if project and err are nil then skip this project
				if err == nil && project == nil {
					return nil
				}
				// Lock the list as only one goroutine should be writing to atlantisConfig.Projects at a time
				lock.Lock()
				defer lock.Unlock()

				log.Info("Created "+projectHcl+" project for ", workingDir)
				atlantisConfig.Projects = append(atlantisConfig.Projects, *project)

				return nil
			})

			if err := errGroup.Wait(); err != nil {
				return nil, nil, err
			}
		}
	}

	// Sort the projects in atlantisConfig by Dir
	sort.Slice(atlantisConfig.Projects, func(i, j int) bool { return atlantisConfig.Projects[i].Dir < atlantisConfig.Projects[j].Dir })
	//
	//if parsingConfig.ExecutionOrderGroups {
	//	projectsMap := make(map[string]*AtlantisProject, len(atlantisConfig.Projects))
	//	for i := range atlantisConfig.Projects {
	//		projectsMap[atlantisConfig.Projects[i].Dir] = &atlantisConfig.Projects[i]
	//	}
	//
	//	// Compute order groups in the cycle to avoid incorrect values in cascade dependencies
	//	hasChanges := true
	//	for i := 0; hasChanges && i <= len(atlantisConfig.Projects); i++ {
	//		hasChanges = false
	//		for _, project := range atlantisConfig.Projects {
	//			executionOrderGroup := 0
	//			// choose order group based on dependencies
	//			for _, dep := range project.Autoplan.WhenModified {
	//				depPath := filepath.Dir(filepath.Join(project.Dir, dep))
	//				if depPath == project.Dir {
	//					// skip dependency on oneself
	//					continue
	//				}
	//
	//				depProject, ok := projectsMap[depPath]
	//				if !ok {
	//					// skip not project dependencies
	//					continue
	//				}
	//				if depProject.ExecutionOrderGroup+1 > executionOrderGroup {
	//					executionOrderGroup = depProject.ExecutionOrderGroup + 1
	//				}
	//			}
	//			if projectsMap[project.Dir].ExecutionOrderGroup != executionOrderGroup {
	//				projectsMap[project.Dir].ExecutionOrderGroup = executionOrderGroup
	//				// repeat the main cycle when changed some project
	//				hasChanges = true
	//			}
	//		}
	//	}
	//
	//	if hasChanges {
	//		// Should be unreachable
	//		log.Warn("Computing execution_order_groups failed. Probably cycle exists")
	//	}
	//
	//	// Sort by execution_order_group
	//	sort.Slice(atlantisConfig.Projects, func(i, j int) bool {
	//		if atlantisConfig.Projects[i].ExecutionOrderGroup == atlantisConfig.Projects[j].ExecutionOrderGroup {
	//			return atlantisConfig.Projects[i].Dir < atlantisConfig.Projects[j].Dir
	//		}
	//		return atlantisConfig.Projects[i].ExecutionOrderGroup < atlantisConfig.Projects[j].ExecutionOrderGroup
	//	})
	//}

	dependsOn := make(map[string][]string)

	for projectName, dependencies := range potentialProjectDependencies {
		for _, dep := range dependencies {
			_, ok := potentialProjectDependencies[dep]
			if ok {
				deps, ok := dependsOn[projectName]
				if ok {
					dependsOn[projectName] = append(deps, dep)
				} else {
					dependsOn[projectName] = []string{dep}
				}
			}
		}
	}

	return &atlantisConfig, dependsOn, nil
}
