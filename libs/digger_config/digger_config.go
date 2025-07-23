package digger_config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samber/lo"

	"github.com/diggerhq/digger/libs/digger_config/terragrunt/atlantis"

	"github.com/dominikbraun/graph"
	"gopkg.in/yaml.v3"
)

type DirWalker interface {
	GetDirs(workingDir string, config DiggerConfigYaml) ([]string, error)
}

type FileSystemTopLevelTerraformDirWalker struct{}

type FileSystemTerragruntDirWalker struct{}

type FileSystemModuleDirWalker struct{}

func ReadDiggerYmlFileContents(dir string) (string, error) {
	var diggerYmlBytes []byte
	diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
	if err != nil {
		// if file doesn't exist look for digger.yaml instead
		slog.Debug("digger.yml not found, trying digger.yaml", "dir", dir)
		diggerYmlBytes, err = os.ReadFile(path.Join(dir, "digger.yaml"))
		if err != nil {
			slog.Error("could not read digger config file",
				"error", err,
				"dir", dir)
			return "", fmt.Errorf("could not read the file both digger.yml and digger.yaml are missing: %v", err)
		}
	}
	diggerYmlStr := string(diggerYmlBytes)
	return diggerYmlStr, nil
}

func CheckOrCreateDiggerFile(dir string) error {
	// Check for digger.yml
	ymlPath := filepath.Join(dir, "digger.yml")
	yamlPath := filepath.Join(dir, "digger.yaml")

	// Check if either file exists
	if _, err := os.Stat(ymlPath); err == nil {
		slog.Debug("digger.yml file exists", "path", ymlPath)
		return nil // digger.yml exists
	}
	if _, err := os.Stat(yamlPath); err == nil {
		slog.Debug("digger.yaml file exists", "path", yamlPath)
		return nil // digger.yaml exists
	}

	// Neither file exists, create digger.yml
	slog.Info("creating new digger.yml file", "path", ymlPath)
	file, err := os.Create(ymlPath)
	if err != nil {
		slog.Error("failed to create digger.yml file", "error", err, "path", ymlPath)
		return err
	}
	defer file.Close()

	// File is created empty by default
	return nil
}

func GetFilesWithExtension(workingDir, ext string) ([]string, error) {
	var files []string
	listOfFiles, err := os.ReadDir(workingDir)
	if err != nil {
		slog.Error("error reading directory",
			"error", err,
			"dir", workingDir)
		return nil, fmt.Errorf("error reading directory %s: %v", workingDir, err)
	}
	for _, f := range listOfFiles {
		if !f.IsDir() {
			r, err := filepath.Match("*"+ext, f.Name())
			if err == nil && r {
				files = append(files, f.Name())
			}
		}
	}

	slog.Debug("found files with extension",
		"extension", ext,
		"dir", workingDir,
		"count", len(files))
	return files, nil
}

func (walker *FileSystemTopLevelTerraformDirWalker) GetDirs(workingDir string, configYaml *DiggerConfigYaml) ([]string, error) {
	var dirs []string
	slog.Debug("searching for terraform directories", "workingDir", workingDir)

	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "modules" {
					slog.Debug("skipping modules directory", "path", path)
					return filepath.SkipDir
				}
				terraformFiles, _ := GetFilesWithExtension(path, ".tf")
				if len(terraformFiles) > 0 {
					relPath := strings.ReplaceAll(path, workingDir+string(os.PathSeparator), "")
					dirs = append(dirs, relPath)
					slog.Debug("found terraform directory", "path", relPath)

					if configYaml.TraverseToNestedProjects != nil && !*configYaml.TraverseToNestedProjects {
						slog.Debug("skipping nested projects", "path", relPath)
						return filepath.SkipDir
					}
				}
			}
			return nil
		})
	if err != nil {
		slog.Error("error walking directories", "error", err, "workingDir", workingDir)
		return nil, err
	}

	slog.Info("found terraform directories", "count", len(dirs), "workingDir", workingDir)
	return dirs, nil
}

func (walker *FileSystemModuleDirWalker) GetDirs(workingDir string, configYaml *DiggerConfigYaml) ([]string, error) {
	var dirs []string
	slog.Debug("searching for module directories", "workingDir", workingDir)

	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == "modules" {
				relPath := strings.ReplaceAll(path, workingDir+string(os.PathSeparator), "")
				dirs = append(dirs, relPath)
				slog.Debug("found modules directory", "path", relPath)
				return filepath.SkipDir
			}
			return nil
		})
	if err != nil {
		slog.Error("error walking directories", "error", err, "workingDir", workingDir)
		return nil, err
	}

	slog.Info("found module directories", "count", len(dirs), "workingDir", workingDir)
	return dirs, nil
}

func (walker *FileSystemTerragruntDirWalker) GetDirs(workingDir string, configYaml *DiggerConfigYaml) ([]string, error) {
	var dirs []string
	slog.Debug("searching for terragrunt directories", "workingDir", workingDir)

	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "modules" {
					slog.Debug("skipping modules directory", "path", path)
					return filepath.SkipDir
				}
				terragruntFiles, _ := GetFilesWithExtension(path, "terragrunt.hcl")
				if len(terragruntFiles) > 0 {
					for _, f := range terragruntFiles {
						terragruntFile := path + string(os.PathSeparator) + f
						fileContent, err := os.ReadFile(terragruntFile)
						if err != nil {
							slog.Error("error reading terragrunt file",
								"error", err,
								"file", terragruntFile)
							return err
						}
						if strings.Contains(string(fileContent), "include \"root\"") {
							relPath := strings.ReplaceAll(path, workingDir+string(os.PathSeparator), "")
							dirs = append(dirs, relPath)
							slog.Debug("found terragrunt directory", "path", relPath)
							return filepath.SkipDir
						}
					}
				}
			}
			return nil
		})
	if err != nil {
		slog.Error("error walking directories", "error", err, "workingDir", workingDir)
		return nil, err
	}

	slog.Info("found terragrunt directories", "count", len(dirs), "workingDir", workingDir)
	return dirs, nil
}

var ErrDiggerConfigConflict = errors.New("more than one digger digger_config file detected, please keep either 'digger.yml' or 'digger.yaml'")

func LoadDiggerConfig(workingDir string, generateProjects bool, changedFiles []string) (*DiggerConfig, *DiggerConfigYaml, graph.Graph[string, Project], error) {
	slog.Info("loading digger configuration",
		"workingDir", workingDir,
		"generateProjects", generateProjects,
		"changedFilesCount", len(changedFiles))

	config := &DiggerConfig{}
	configYaml, err := LoadDiggerConfigYaml(workingDir, generateProjects, changedFiles)
	if err != nil {
		slog.Error("failed to load digger config YAML", "error", err, "workingDir", workingDir)
		return nil, nil, nil, err
	}

	config, projectDependencyGraph, err := ConvertDiggerYamlToConfig(configYaml)
	if err != nil {
		slog.Error("failed to convert YAML to config", "error", err)
		return nil, nil, nil, err
	}

	err = ValidateDiggerConfig(config)
	if err != nil {
		slog.Warn("digger config validation failed", "error", err)
		return config, configYaml, projectDependencyGraph, err
	}

	slog.Info("digger configuration loaded successfully",
		"projectCount", len(config.Projects),
		"workflowCount", len(config.Workflows))
	return config, configYaml, projectDependencyGraph, nil
}

func LoadDiggerConfigFromString(yamlString, terraformDir string) (*DiggerConfig, *DiggerConfigYaml, graph.Graph[string, Project], error) {
	slog.Info("loading digger configuration from string", "terraformDir", terraformDir)

	config := &DiggerConfig{}
	configYaml, err := LoadDiggerConfigYamlFromString(yamlString)
	if err != nil {
		slog.Error("failed to load digger config YAML from string", "error", err)
		return nil, nil, nil, err
	}

	err = ValidateDiggerConfigYaml(configYaml, "loaded_yaml_string")
	if err != nil {
		slog.Error("digger config YAML validation failed", "error", err)
		return nil, nil, nil, err
	}

	err = HandleYamlProjectGeneration(configYaml, terraformDir, nil)
	if err != nil {
		slog.Error("failed to handle project generation", "error", err)
		return nil, nil, nil, err
	}

	config, projectDependencyGraph, err := ConvertDiggerYamlToConfig(configYaml)
	if err != nil {
		slog.Error("failed to convert YAML to config", "error", err)
		return nil, nil, nil, err
	}

	err = ValidateDiggerConfig(config)
	if err != nil {
		slog.Warn("digger config validation failed", "error", err)
		return config, configYaml, projectDependencyGraph, err
	}

	slog.Info("digger configuration loaded successfully from string",
		"projectCount", len(config.Projects),
		"workflowCount", len(config.Workflows))
	return config, configYaml, projectDependencyGraph, nil
}

func LoadDiggerConfigYamlFromString(yamlString string) (*DiggerConfigYaml, error) {
	configYaml := &DiggerConfigYaml{}
	if err := yaml.Unmarshal([]byte(yamlString), configYaml); err != nil {
		slog.Error("error parsing YAML", "error", err)
		return nil, fmt.Errorf("error parsing yaml: %v", err)
	}

	slog.Debug("successfully loaded digger config YAML from string")
	return configYaml, nil
}

func validateBlockYaml(blocks []BlockYaml) error {
	for _, b := range blocks {
		if b.Terragrunt {
			if b.RootDir == nil {
				slog.Error("terragrunt block missing root_dir", "blockName", b.BlockName)
				return fmt.Errorf("block %v is a terragrunt block but does not have root_dir specified", b.BlockName)
			}
		}
	}
	return nil
}

func checkBlockInChangedFiles(dir string, changedFiles []string) bool {
	if changedFiles == nil {
		return true
	}
	for _, file := range changedFiles {
		if strings.HasPrefix(NormalizeFileName(file), NormalizeFileName(dir)) {
			slog.Debug("found changed file in block directory",
				"dir", dir,
				"file", file)
			return true
		}
	}
	slog.Debug("no changed files found in block directory", "dir", dir)
	return false
}

func HandleYamlProjectGeneration(config *DiggerConfigYaml, terraformDir string, changedFiles []string) error {
	os.Setenv("DIGGER_GENERATE_PROJECT", "true")
	defer os.Unsetenv("DIGGER_GENERATE_PROJECT")
	if config.GenerateProjectsConfig != nil && config.GenerateProjectsConfig.TerragruntParsingConfig != nil {
		slog.Warn("terragrunt generation using top level config is deprecated",
			"recommendation", "https://docs.digger.dev/howto/generate-projects#blocks-syntax-with-terragrunt")

		err := hydrateDiggerConfigYamlWithTerragrunt(config, *config.GenerateProjectsConfig.TerragruntParsingConfig, terraformDir, "")
		if err != nil {
			slog.Error("failed to hydrate config with terragrunt", "error", err)
			return err
		}
	} else if config.GenerateProjectsConfig != nil && config.GenerateProjectsConfig.Terragrunt {
		slog.Warn("terragrunt generation using top level config is deprecated",
			"recommendation", "https://docs.digger.dev/howto/generate-projects#blocks-syntax-with-terragrunt")

		err := hydrateDiggerConfigYamlWithTerragrunt(config, TerragruntParsingConfig{}, terraformDir, "")
		if err != nil {
			slog.Error("failed to hydrate config with terragrunt", "error", err)
			return err
		}
	} else if config.GenerateProjectsConfig != nil {
		dirWalker := &FileSystemTopLevelTerraformDirWalker{}

		slog.Info("finding terraform directories for project generation", "terraformDir", terraformDir)
		dirs, err := dirWalker.GetDirs(terraformDir, config)
		if err != nil {
			slog.Error("error walking through directories", "error", err, "terraformDir", terraformDir)
			return fmt.Errorf("error while walking through directories: %v", err)
		}

		var includePatterns []string
		var excludePatterns []string
		if config.GenerateProjectsConfig.Include != "" || config.GenerateProjectsConfig.Exclude != "" {
			includePatterns = []string{config.GenerateProjectsConfig.Include}
			excludePatterns = []string{config.GenerateProjectsConfig.Exclude}

			slog.Info("generating projects with include/exclude patterns",
				"include", config.GenerateProjectsConfig.Include,
				"exclude", config.GenerateProjectsConfig.Exclude)

			for _, dir := range dirs {
				if MatchIncludeExcludePatternsToFile(dir, includePatterns, excludePatterns) {
					projectName := strings.ReplaceAll(dir, "/", "_")

					slog.Debug("creating project for directory",
						"dir", dir,
						"projectName", projectName)

					project := ProjectYaml{
						Name:                 projectName,
						Dir:                  dir,
						Workflow:             defaultWorkflowName,
						Workspace:            "default",
						AwsRoleToAssume:      config.GenerateProjectsConfig.AwsRoleToAssume,
						Generated:            true,
						AwsCognitoOidcConfig: config.GenerateProjectsConfig.AwsCognitoOidcConfig,
					}
					config.Projects = append(config.Projects, &project)
				}
			}
		}
		if config.GenerateProjectsConfig.Blocks != nil && len(config.GenerateProjectsConfig.Blocks) > 0 {
			slog.Info("processing project generation blocks",
				"blockCount", len(config.GenerateProjectsConfig.Blocks))

			err = validateBlockYaml(config.GenerateProjectsConfig.Blocks)
			if err != nil {
				slog.Error("block validation failed", "error", err)
				return err
			}

			// if blocks of include/exclude patterns defined
			for _, b := range config.GenerateProjectsConfig.Blocks {
				if b.Terragrunt {
					if checkBlockInChangedFiles(*b.RootDir, changedFiles) {
						slog.Info("generating terragrunt projects for block",
							"blockName", b.BlockName,
							"rootDir", *b.RootDir)

						workflow := "default"
						if b.Workflow != "" {
							workflow = b.Workflow
						}

						// load the parsing config and override the block values
						tgParsingConfig := b.TerragruntParsingConfig
						if tgParsingConfig == nil {
							tgParsingConfig = &TerragruntParsingConfig{}
						}
						tgParsingConfig.CreateProjectName = true
						tgParsingConfig.DefaultWorkflow = workflow
						tgParsingConfig.WorkflowFile = b.WorkflowFile
						tgParsingConfig.FilterPath = path.Join(terraformDir, *b.RootDir)
						tgParsingConfig.AwsRoleToAssume = b.AwsRoleToAssume
						tgParsingConfig.AwsCognitoOidcConfig = b.AwsCognitoOidcConfig

						err := hydrateDiggerConfigYamlWithTerragrunt(config, *tgParsingConfig, terraformDir, b.BlockName)
						if err != nil {
							slog.Error("failed to hydrate config with terragrunt",
								"error", err,
								"blockName", b.BlockName)
							return err
						}
					} else {
						slog.Debug("skipping block due to no changed files", "blockName", b.BlockName)
					}
				} else {
					includePatterns = []string{b.Include}
					excludePatterns = []string{b.Exclude}
					workflow := "default"
					if b.Workflow != "" {
						workflow = b.Workflow
					}

					workspace := "default"
					if b.Workspace != "" {
						workspace = b.Workspace
					}

					slog.Info("generating terraform projects for block",
						"blockName", b.BlockName,
						"include", b.Include,
						"exclude", b.Exclude)

					for _, dir := range dirs {
						if MatchIncludeExcludePatternsToFile(dir, includePatterns, excludePatterns) {
							projectName := strings.ReplaceAll(dir, "/", "_")

							slog.Debug("creating project for directory",
								"blockName", b.BlockName,
								"dir", dir,
								"projectName", projectName)

							project := ProjectYaml{
								Name:                 projectName,
								Dir:                  dir,
								Workflow:             workflow,
								Workspace:            workspace,
								OpenTofu:             b.OpenTofu,
								AwsRoleToAssume:      b.AwsRoleToAssume,
								Generated:            true,
								AwsCognitoOidcConfig: b.AwsCognitoOidcConfig,
								WorkflowFile:         b.WorkflowFile,
								IncludePatterns:      b.IncludePatterns,
								ExcludePatterns:      b.ExcludePatterns,
							}
							config.Projects = append(config.Projects, &project)
						}
					}
				}
			}
		}
	}

	slog.Info("completed project generation",
		"projectCount", len(config.Projects),
		"terraformDir", terraformDir)
	return nil
}

func LoadDiggerConfigYaml(workingDir string, generateProjects bool, changedFiles []string) (*DiggerConfigYaml, error) {
	slog.Info("loading digger config YAML",
		"workingDir", workingDir,
		"generateProjects", generateProjects,
		"changedFilesCount", len(changedFiles))

	configYaml := &DiggerConfigYaml{}
	fileName, err := retrieveConfigFile(workingDir)
	if err != nil {
		if errors.Is(err, ErrDiggerConfigConflict) {
			slog.Error("config file conflict detected", "error", err, "workingDir", workingDir)
			return nil, fmt.Errorf("error while retrieving digger_config file: %v", err)
		}
	}

	if fileName == "" {
		slog.Error("digger config file not found", "workingDir", workingDir)
		return nil, fmt.Errorf("could not find digger.yml or digger.yaml in root of repository")
	} else {
		slog.Debug("reading digger config file", "fileName", fileName)
		data, err := os.ReadFile(fileName)
		if err != nil {
			slog.Error("failed to read config file", "error", err, "fileName", fileName)
			return nil, fmt.Errorf("failed to read digger_config file %s: %v", fileName, err)
		}

		if err := yaml.Unmarshal(data, configYaml); err != nil {
			slog.Error("error parsing YAML", "error", err, "fileName", fileName)
			return nil, fmt.Errorf("error parsing '%s': %v", fileName, err)
		}
	}

	err = ValidateDiggerConfigYaml(configYaml, fileName)
	if err != nil {
		slog.Error("config validation failed", "error", err, "fileName", fileName)
		return configYaml, err
	}

	if generateProjects {
		slog.Info("generating projects from config", "fileName", fileName)
		err = HandleYamlProjectGeneration(configYaml, workingDir, changedFiles)
		if err != nil {
			slog.Error("project generation failed", "error", err)
			return configYaml, err
		}
	}

	slog.Info("successfully loaded digger config YAML",
		"fileName", fileName,
		"projectCount", len(configYaml.Projects),
		"workflowCount", len(configYaml.Workflows))
	return configYaml, nil
}

func ValidateDiggerConfigYaml(configYaml *DiggerConfigYaml, fileName string) error {
	slog.Debug("validating digger config YAML", "fileName", fileName)

	if configYaml.DependencyConfiguration != nil {
		if configYaml.DependencyConfiguration.Mode != DependencyConfigurationHard && configYaml.DependencyConfiguration.Mode != DependencyConfigurationSoft {
			slog.Error("invalid dependency configuration mode",
				"mode", configYaml.DependencyConfiguration.Mode,
				"validModes", []string{string(DependencyConfigurationHard), string(DependencyConfigurationSoft)})
			return fmt.Errorf("dependency digger_config mode can only be '%s' or '%s'", DependencyConfigurationHard, DependencyConfigurationSoft)
		}
	}

	if configYaml.Workflows != nil {
		for _, workflow := range configYaml.Workflows {
			if workflow == nil {
				continue
			}
			if workflow.Plan != nil && workflow.Plan.FilterRegex != nil {
				_, err := regexp.Compile(*workflow.Plan.FilterRegex)
				if err != nil {
					slog.Error("invalid regex for plan filter",
						"regex", *workflow.Plan.FilterRegex,
						"error", err)
					return fmt.Errorf("regex for plan filter is invalid: %v", err)
				}
			}
			if workflow.Apply != nil && workflow.Apply.FilterRegex != nil {
				_, err := regexp.Compile(*workflow.Apply.FilterRegex)
				if err != nil {
					slog.Error("invalid regex for apply filter",
						"regex", *workflow.Apply.FilterRegex,
						"error", err)
					return fmt.Errorf("regex for apply filter is invalid: %v", err)
				}
			}
		}
	}
	if configYaml.GenerateProjectsConfig != nil {
		if configYaml.GenerateProjectsConfig.Include != "" &&
			configYaml.GenerateProjectsConfig.Exclude != "" &&
			len(configYaml.GenerateProjectsConfig.Blocks) != 0 {
			slog.Error("conflicting project generation configuration",
				"include", configYaml.GenerateProjectsConfig.Include,
				"exclude", configYaml.GenerateProjectsConfig.Exclude,
				"blockCount", len(configYaml.GenerateProjectsConfig.Blocks))
			return fmt.Errorf("if include/exclude patterns are used for project generation, blocks of include/exclude can't be used")
		}
	}

	slog.Debug("digger config YAML validation successful", "fileName", fileName)
	return nil
}

func checkThatOnlyOneIacSpecifiedPerProject(project *Project) error {
	slog.Debug("checking IAC configuration for project", "projectName", project.Name)

	nOfIac := 0
	if project.Terragrunt {
		nOfIac++
	}
	if project.OpenTofu {
		nOfIac++
	}
	if project.Pulumi {
		nOfIac++
	}
	if nOfIac > 1 {
		slog.Error("multiple IAC types defined for project",
			"projectName", project.Name,
			"terragrunt", project.Terragrunt,
			"openTofu", project.OpenTofu,
			"pulumi", project.Pulumi)
		return fmt.Errorf("project %v has more than one IAC defined, please specify one of terragrunt or pulumi or opentofu", project.Name)
	}
	return nil
}

func validatePulumiProject(project *Project) error {
	if project.Pulumi {
		slog.Debug("validating pulumi project configuration", "projectName", project.Name)
		if project.PulumiStack == "" {
			slog.Error("pulumi stack not specified for project", "projectName", project.Name)
			return fmt.Errorf("for pulumi project %v you must specify a pulumi stack", project.Name)
		}
	}
	return nil
}

func ValidateProjects(config *DiggerConfig) error {
	slog.Debug("validating projects configuration", "projectCount", len(config.Projects))

	projects := config.Projects
	for _, project := range projects {
		err := checkThatOnlyOneIacSpecifiedPerProject(&project)
		if err != nil {
			return err
		}

		err = validatePulumiProject(&project)
		if err != nil {
			return err
		}
	}

	slog.Debug("projects validation successful")
	return nil
}

func ValidateDiggerConfig(config *DiggerConfig) error {
	slog.Info("validating digger configuration",
		"projectCount", len(config.Projects),
		"workflowCount", len(config.Workflows))

	err := ValidateProjects(config)
	if err != nil {
		return err
	}

	if config.CommentRenderMode != CommentRenderModeBasic && config.CommentRenderMode != CommentRenderModeGroupByModule {
		slog.Error("invalid comment render mode",
			"mode", config.CommentRenderMode,
			"validModes", []string{string(CommentRenderModeBasic), string(CommentRenderModeGroupByModule)})
		return fmt.Errorf("invalid value for comment_render_mode, %v expecting %v, %v", config.CommentRenderMode, CommentRenderModeBasic, CommentRenderModeGroupByModule)
	}

	for _, p := range config.Projects {
		_, ok := config.Workflows[p.Workflow]
		if !ok {
			slog.Error("workflow not found for project",
				"projectName", p.Name,
				"workflow", p.Workflow)
			return fmt.Errorf("failed to find workflow digger_config '%s' for project '%s'", p.Workflow, p.Name)
		}
	}

	for name, w := range config.Workflows {
		for i, s := range w.Plan.Steps {
			if s.Action == "" {
				slog.Error("empty action in plan step",
					"workflowName", name,
					"stepIndex", i)
				return fmt.Errorf("plan step's action can't be empty")
			}
		}
	}

	for name, w := range config.Workflows {
		for i, s := range w.Apply.Steps {
			if s.Action == "" {
				slog.Error("empty action in apply step",
					"workflowName", name,
					"stepIndex", i)
				return fmt.Errorf("apply step's action can't be empty")
			}
		}
	}

	slog.Info("digger configuration validation successful")
	return nil
}

func hydrateDiggerConfigYamlWithTerragrunt(configYaml *DiggerConfigYaml, parsingConfig TerragruntParsingConfig, workingDir, blockName string) error {
	slog.Info("hydrating config with terragrunt projects",
		"workingDir", workingDir,
		"filterPath", parsingConfig.FilterPath)

	root := workingDir
	if parsingConfig.GitRoot != nil {
		root = path.Join(workingDir, *parsingConfig.GitRoot)
		slog.Debug("using custom git root", "root", root)
	}
	projectExternalChilds := true

	if parsingConfig.CreateHclProjectExternalChilds != nil {
		projectExternalChilds = *parsingConfig.CreateHclProjectExternalChilds
	}

	parallel := true
	if parsingConfig.Parallel != nil {
		parallel = *parsingConfig.Parallel
	}

	ignoreParentTerragrunt := true
	if parsingConfig.IgnoreParentTerragrunt != nil {
		ignoreParentTerragrunt = *parsingConfig.IgnoreParentTerragrunt
	}

	cascadeDependencies := true
	if parsingConfig.CascadeDependencies != nil {
		cascadeDependencies = *parsingConfig.CascadeDependencies
	}

	executionOrderGroups := false
	if parsingConfig.ExecutionOrderGroups != nil {
		executionOrderGroups = *parsingConfig.ExecutionOrderGroups
	}

	workflowFile := "digger_workflow.yml"
	if parsingConfig.WorkflowFile != "" {
		workflowFile = parsingConfig.WorkflowFile
	}

	slog.Debug("parsing terragrunt configuration",
		"root", root,
		"defaultWorkflow", parsingConfig.DefaultWorkflow,
		"filterPath", parsingConfig.FilterPath)

	atlantisConfig, projectDependsOnMap, err := atlantis.Parse(
		root,
		parsingConfig.ProjectHclFiles,
		projectExternalChilds,
		parsingConfig.AutoMerge,
		parallel,
		parsingConfig.FilterPath,
		parsingConfig.CreateHclProjectChilds,
		ignoreParentTerragrunt,
		parsingConfig.IgnoreDependencyBlocks,
		cascadeDependencies,
		parsingConfig.DefaultWorkflow,
		parsingConfig.DefaultApplyRequirements,
		parsingConfig.AutoPlan,
		parsingConfig.DefaultTerraformVersion,
		parsingConfig.CreateProjectName,
		parsingConfig.CreateWorkspace,
		parsingConfig.PreserveProjects,
		parsingConfig.UseProjectMarkers,
		executionOrderGroups,
		parsingConfig.TriggerProjectsFromDirOnly,
	)
	if err != nil {
		slog.Error("failed to parse terragrunt configuration", "error", err)
		return fmt.Errorf("failed to autogenerate digger_config, error during parse: %v", err)
	}

	if atlantisConfig.Projects == nil {
		slog.Error("atlantis projects configuration is nil")
		return fmt.Errorf("atlantisConfig.Projects is nil")
	}

	configYaml.AutoMerge = &atlantisConfig.AutoMerge

	pathPrefix := ""
	if parsingConfig.GitRoot != nil {
		pathPrefix = *parsingConfig.GitRoot
	}

	slog.Info("found terragrunt projects",
		"count", len(atlantisConfig.Projects),
		"pathPrefix", pathPrefix)

	for _, atlantisProject := range atlantisConfig.Projects {
		// normalize paths
		projectDir := path.Join(pathPrefix, atlantisProject.Dir)
		atlantisProject.Autoplan.WhenModified, err = GetPatternsRelativeToRepo(projectDir, atlantisProject.Autoplan.WhenModified)
		if err != nil {
			slog.Error("could not normalize patterns",
				"error", err,
				"projectDir", projectDir)
			return fmt.Errorf("could not normalize patterns: %v", err)
		}

		slog.Debug("adding terragrunt project",
			"projectName", atlantisProject.Name,
			"projectDir", projectDir,
			"workspace", atlantisProject.Workspace)

		diggerProject := &ProjectYaml{
			BlockName:            blockName,
			Name:                 atlantisProject.Name,
			Dir:                  projectDir,
			Workspace:            atlantisProject.Workspace,
			Terragrunt:           true,
			Workflow:             atlantisProject.Workflow,
			WorkflowFile:         workflowFile,
			IncludePatterns:      atlantisProject.Autoplan.WhenModified,
			Generated:            true,
			AwsRoleToAssume:      parsingConfig.AwsRoleToAssume,
			AwsCognitoOidcConfig: parsingConfig.AwsCognitoOidcConfig,
		}

		if parsingConfig.DependsOnOrdering != nil && *parsingConfig.DependsOnOrdering {
			diggerProject.DependencyProjects = projectDependsOnMap[atlantisProject.Name]
		}

		configYaml.Projects = append(configYaml.Projects, diggerProject)
	}

	slog.Info("completed hydrating config with terragrunt projects",
		"totalProjectCount", len(configYaml.Projects))
	return nil
}

func (c *DiggerConfig) GetProject(projectName string) *Project {
	for _, project := range c.Projects {
		if projectName == project.Name {
			return &project
		}
	}
	slog.Debug("project not found by name", "projectName", projectName)
	return nil
}

func (c *DiggerConfig) GetProjects(projectName string) []Project {
	if projectName == "" {
		slog.Debug("returning all projects", "count", len(c.Projects))
		return c.Projects
	}

	project := c.GetProject(projectName)
	if project == nil {
		slog.Debug("no project found for name", "projectName", projectName)
		return nil
	}

	slog.Debug("found project by name", "projectName", projectName)
	return []Project{*project}
}

type ProjectToSourceMapping struct {
	ImpactingLocations []string          `json:"impacting_locations"`
	CommentIds         map[string]string `json:"comment_ids"` // impactingLocation => PR commentId
}

func (c *DiggerConfig) GetModifiedProjects(changedFiles []string) ([]Project, map[string]ProjectToSourceMapping) {
	slog.Info("finding modified projects", "changedFilesCount", len(changedFiles))

	var result []Project
	mapping := make(map[string]ProjectToSourceMapping)

	for _, project := range c.Projects {
		sourceChangesForProject := make([]string, 0)
		isProjectAdded := false

		for _, changedFile := range changedFiles {
			includePatterns := project.IncludePatterns
			excludePatterns := project.ExcludePatterns

			includePatterns = append(includePatterns, filepath.Join(project.Dir, "*"))

			// all our patterns are the globale dir pattern + the include patterns specified by user
			if MatchIncludeExcludePatternsToFile(changedFile, includePatterns, excludePatterns) {
				if !isProjectAdded {
					result = append(result, project)
					isProjectAdded = true
					slog.Debug("adding project to modified list",
						"projectName", project.Name,
						"dir", project.Dir)
				}

				changedDir := filepath.Dir(changedFile)
				if !lo.Contains(sourceChangesForProject, changedDir) {
					sourceChangesForProject = append(sourceChangesForProject, changedDir)
					slog.Debug("adding source change directory for project",
						"projectName", project.Name,
						"changedDir", changedDir)
				}
			}
		}

		mapping[project.Name] = ProjectToSourceMapping{
			ImpactingLocations: sourceChangesForProject,
		}
	}

	slog.Info("found modified projects",
		"count", len(result),
		"changedFilesCount", len(changedFiles))
	return result, mapping
}

func (c *DiggerConfig) GetDirectory(projectName string) string {
	project := c.GetProject(projectName)
	if project == nil {
		slog.Debug("no directory found for project", "projectName", projectName)
		return ""
	}

	slog.Debug("found directory for project",
		"projectName", projectName,
		"dir", project.Dir)
	return project.Dir
}

func (c *DiggerConfig) GetWorkflow(workflowName string) *Workflow {
	workflows := c.Workflows

	workflow, ok := workflows[workflowName]
	if !ok {
		slog.Debug("workflow not found", "workflowName", workflowName)
		return nil
	}

	slog.Debug("found workflow", "workflowName", workflowName)
	return &workflow
}

type File struct {
	Filename string
}

func isFileExists(path string) bool {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		slog.Debug("file does not exist", "path", path)
		return false
	}
	// file exists make sure it's not a directory
	isFile := !fi.IsDir()
	slog.Debug("file existence check", "path", path, "exists", isFile)
	return isFile
}

func retrieveConfigFile(workingDir string) (string, error) {
	fileName := "digger"
	customConfigFile := os.Getenv("DIGGER_FILENAME") != ""

	if customConfigFile {
		fileName = os.Getenv("DIGGER_FILENAME")
		slog.Debug("using custom config file", "fileName", fileName)
	}

	if workingDir != "" {
		fileName = path.Join(workingDir, fileName)
	}

	if !customConfigFile {
		// Make sure we don't have more than one digger digger_config file
		ymlCfg := fileName + ".yml"
		yamlCfg := fileName + ".yaml"

		slog.Debug("checking config file existence",
			"ymlPath", ymlCfg,
			"yamlPath", yamlCfg)

		ymlCfgExists := isFileExists(ymlCfg)
		yamlCfgExists := isFileExists(yamlCfg)

		if ymlCfgExists && yamlCfgExists {
			slog.Error("config file conflict detected",
				"ymlPath", ymlCfg,
				"yamlPath", yamlCfg)
			return "", ErrDiggerConfigConflict
		} else if ymlCfgExists {
			slog.Info("using yml config file", "path", ymlCfg)
			return ymlCfg, nil
		} else if yamlCfgExists {
			slog.Info("using yaml config file", "path", yamlCfg)
			return yamlCfg, nil
		}
	} else {
		slog.Info("using custom config file", "path", fileName)
		return fileName, nil
	}

	// Passing this point means digger digger_config file is
	// missing which is a non-error
	slog.Warn("no config file found", "workingDir", workingDir)
	return "", nil
}

func CollectTerraformEnvConfig(envs *TerraformEnvConfig, performInterpolation bool) (map[string]string, map[string]string) {
	stateEnvVars := map[string]string{}
	commandEnvVars := map[string]string{}

	if envs != nil {
		for _, envvar := range envs.State {
			if envvar.Value != "" {
				stateEnvVars[envvar.Name] = envvar.Value
			} else if envvar.ValueFrom != "" {
				if performInterpolation {
					stateEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
				} else {
					stateEnvVars[envvar.Name] = fmt.Sprintf("$DIGGER_%v", envvar.ValueFrom)
				}
			}
		}

		for _, envvar := range envs.Commands {
			if envvar.Value != "" {
				commandEnvVars[envvar.Name] = envvar.Value
			} else if envvar.ValueFrom != "" {
				if performInterpolation {
					commandEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
				} else {
					commandEnvVars[envvar.Name] = fmt.Sprintf("$DIGGER_%v", envvar.ValueFrom)
				}
			}
		}
	}

	return stateEnvVars, commandEnvVars
}
