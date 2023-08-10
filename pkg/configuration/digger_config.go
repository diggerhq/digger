package configuration

import (
	"digger/pkg/terragrunt/atlantis"
	"digger/pkg/utils"
	"errors"
	"fmt"
	"github.com/dominikbraun/graph"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type DirWalker interface {
	GetDirs(workingDir string) ([]string, error)
}

type FileSystemTopLevelTerraformDirWalker struct {
}

type FileSystemTerragruntDirWalker struct {
}

type FileSystemModuleDirWalker struct {
}

func GetFilesWithExtension(workingDir string, ext string) ([]string, error) {
	var files []string
	listOfFiles, err := os.ReadDir(workingDir)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error reading directory %s: %v", workingDir, err))
	}
	for _, f := range listOfFiles {
		if !f.IsDir() {
			r, err := regexp.MatchString(ext, f.Name())
			if err == nil && r {
				files = append(files, f.Name())
			}
		}
	}

	return files, nil
}

func (walker *FileSystemTopLevelTerraformDirWalker) GetDirs(workingDir string) ([]string, error) {
	var dirs []string
	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "modules" {
					return filepath.SkipDir
				}
				terraformFiles, _ := GetFilesWithExtension(path, ".tf")
				if len(terraformFiles) > 0 {
					dirs = append(dirs, strings.ReplaceAll(path, workingDir+string(os.PathSeparator), ""))
					return filepath.SkipDir
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

func (walker *FileSystemModuleDirWalker) GetDirs(workingDir string) ([]string, error) {
	var dirs []string
	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == "modules" {
				dirs = append(dirs, strings.ReplaceAll(path, workingDir+string(os.PathSeparator), ""))
				return filepath.SkipDir
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

func (walker *FileSystemTerragruntDirWalker) GetDirs(workingDir string) ([]string, error) {
	var dirs []string
	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "modules" {
					return filepath.SkipDir
				}
				terragruntFiles, _ := GetFilesWithExtension(path, "terragrunt.hcl")
				if len(terragruntFiles) > 0 {
					for _, f := range terragruntFiles {
						terragruntFile := path + string(os.PathSeparator) + f
						fileContent, err := os.ReadFile(terragruntFile)
						if err != nil {
							return err
						}
						if strings.Contains(string(fileContent), "include \"root\"") {
							dirs = append(dirs, strings.ReplaceAll(path, workingDir+string(os.PathSeparator), ""))
							return filepath.SkipDir
						}
					}
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

var ErrDiggerConfigConflict = errors.New("more than one digger config file detected, please keep either 'digger.yml' or 'digger.yaml'")

func LoadDiggerConfig(workingDir string) (*DiggerConfig, graph.Graph[string, string], error) {
	configYaml := &DiggerConfigYaml{}
	config := &DiggerConfig{}
	fileName, err := retrieveConfigFile(workingDir)
	if err != nil {
		if errors.Is(err, ErrDiggerConfigConflict) {
			return nil, nil, fmt.Errorf("error while retrieving config file: %v", err)
		}
	}

	if fileName == "" {
		configYaml, err = AutoDetectDiggerConfig(workingDir)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to auto detect digger config: %v", err)
		}
		marshalledConfig, err := yaml.Marshal(configYaml)
		if err != nil {
			log.Printf("failed to marshal auto detected digger config: %v", err)
		} else {
			log.Printf("Auto detected digger config: \n%v", marshalledConfig)
		}
	} else {
		data, err := os.ReadFile(fileName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read config file %s: %v", fileName, err)
		}

		if err := yaml.Unmarshal(data, configYaml); err != nil {
			return nil, nil, fmt.Errorf("error parsing '%s': %v", fileName, err)
		}
	}

	if configYaml.GenerateProjectsConfig != nil && configYaml.GenerateProjectsConfig.TerragruntParsingConfig != nil {
		hydrateDiggerConfig(configYaml)
	}

	if (configYaml.Projects == nil || len(configYaml.Projects) == 0) && configYaml.GenerateProjectsConfig == nil {
		return nil, nil, fmt.Errorf("no projects configuration found in '%s'", fileName)
	}

	config, projectDependencyGraph, err := ConvertDiggerYamlToConfig(configYaml, workingDir)
	if err != nil {
		return nil, nil, err
	}

	for _, p := range config.Projects {
		_, ok := config.Workflows[p.Workflow]
		if !ok {
			return nil, nil, fmt.Errorf("failed to find workflow config '%s' for project '%s'", p.Workflow, p.Name)
		}
	}

	for _, w := range config.Workflows {
		for _, s := range w.Plan.Steps {
			if s.Action == "" {
				return nil, nil, fmt.Errorf("plan step's action can't be empty")
			}
		}
	}

	for _, w := range config.Workflows {
		for _, s := range w.Apply.Steps {
			if s.Action == "" {
				return nil, nil, fmt.Errorf("apply step's action can't be empty")
			}
		}
	}
	return config, projectDependencyGraph, nil
}

func hydrateDiggerConfig(configYaml *DiggerConfigYaml) {
	parsingConfig := configYaml.GenerateProjectsConfig.TerragruntParsingConfig
	root := ""
	if parsingConfig.GitRoot == nil {
		wd, err := os.Getwd()
		if err != nil {
			log.Printf("failed to get working directory: %v", err)
		} else {
			root = wd
		}
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

	atlantisConfig, _, err := atlantis.Parse(
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
	)
	if err != nil {
		log.Printf("failed to autogenerate config: %v", err)
	}

	configYaml.AutoMerge = &atlantisConfig.AutoMerge
	for _, atlantisProject := range atlantisConfig.Projects {
		configYaml.Projects = append(configYaml.Projects, &ProjectYaml{
			Name:            atlantisProject.Name,
			Dir:             atlantisProject.Dir,
			Workspace:       atlantisProject.Workspace,
			Terragrunt:      true,
			Workflow:        atlantisProject.Workflow,
			IncludePatterns: atlantisProject.Autoplan.WhenModified,
		})
	}
}

func AutoDetectDiggerConfig(workingDir string) (*DiggerConfigYaml, error) {
	configYaml := &DiggerConfigYaml{}
	collectUsageData := true
	configYaml.CollectUsageData = &collectUsageData

	terragruntDirWalker := &FileSystemTerragruntDirWalker{}
	terraformDirWalker := &FileSystemTopLevelTerraformDirWalker{}
	moduleDirWalker := &FileSystemModuleDirWalker{}

	terragruntDirs, err := terragruntDirWalker.GetDirs(workingDir)

	if err != nil {
		return nil, err
	}

	terraformDirs, err := terraformDirWalker.GetDirs(workingDir)

	if err != nil {
		return nil, err
	}

	moduleDirs, err := moduleDirWalker.GetDirs(workingDir)

	modulePatterns := []string{}
	for _, dir := range moduleDirs {
		modulePatterns = append(modulePatterns, dir+"/**")
	}

	if err != nil {
		return nil, err
	}
	if len(terragruntDirs) > 0 {
		// TODO: add support for dependency graph when parsing terragrunt config
		for _, dir := range terragruntDirs {
			project := ProjectYaml{Name: dir, Dir: dir, Workflow: defaultWorkflowName, Workspace: "default", Terragrunt: true, IncludePatterns: modulePatterns}
			configYaml.Projects = append(configYaml.Projects, &project)
		}
		return configYaml, nil
	} else if len(terraformDirs) > 0 {
		for _, dir := range terraformDirs {
			project := ProjectYaml{Name: dir, Dir: dir, Workflow: defaultWorkflowName, Workspace: "default", Terragrunt: false, IncludePatterns: modulePatterns}
			configYaml.Projects = append(configYaml.Projects, &project)
		}
		return configYaml, nil
	} else {
		return nil, fmt.Errorf("no terragrunt or terraform project detected in the repository")
	}
}

func (c *DiggerConfig) GetProject(projectName string) *Project {
	for _, project := range c.Projects {
		if projectName == project.Name {
			return &project
		}
	}
	return nil
}

func (c *DiggerConfig) GetProjects(projectName string) []Project {
	if projectName == "" {
		return c.Projects
	}
	project := c.GetProject(projectName)
	if project == nil {
		return nil
	}
	return []Project{*project}
}

func (c *DiggerConfig) GetModifiedProjects(changedFiles []string) []Project {
	var result []Project
	for _, project := range c.Projects {
		for _, changedFile := range changedFiles {
			// we append ** to make our directory a globable pattern
			projectDirPattern := path.Join(project.Dir, "**")
			includePatterns := project.IncludePatterns
			excludePatterns := project.ExcludePatterns
			fmt.Printf("!!!!!!!!!!!!!!!!!!!!!!!! project name %v, project dir %v, changed file %v\n", project.Name, project.Dir, changedFile)
			// all our patterns are the globale dir pattern + the include patterns specified by user
			allIncludePatterns := append([]string{projectDirPattern}, includePatterns...)
			if utils.MatchIncludeExcludePatternsToFile(changedFile, allIncludePatterns, excludePatterns) {
				result = append(result, project)
				break
			}
		}
	}
	return result
}

func (c *DiggerConfig) GetDirectory(projectName string) string {
	project := c.GetProject(projectName)
	if project == nil {
		return ""
	}
	return project.Dir
}

func (c *DiggerConfig) GetWorkflow(workflowName string) *Workflow {
	workflows := c.Workflows

	workflow, ok := workflows[workflowName]
	if !ok {
		return nil
	}
	return &workflow

}

type File struct {
	Filename string
}

func isFileExists(path string) bool {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	// file exists make sure it's not a directory
	return !fi.IsDir()
}

func retrieveConfigFile(workingDir string) (string, error) {
	fileName := "digger"
	if workingDir != "" {
		fileName = path.Join(workingDir, fileName)
	}

	// Make sure we don't have more than one digger config file
	ymlCfg := isFileExists(fileName + ".yml")
	yamlCfg := isFileExists(fileName + ".yaml")
	if ymlCfg && yamlCfg {
		return "", ErrDiggerConfigConflict
	}

	// At this point we know there are no duplicates
	// Return the first one that exists
	if ymlCfg {
		return path.Join(workingDir, "digger.yml"), nil
	}
	if yamlCfg {
		return path.Join(workingDir, "digger.yaml"), nil
	}

	// Passing this point means digger config file is
	// missing which is a non-error
	return "", nil
}

func CollectTerraformEnvConfig(envs *TerraformEnvConfig) (map[string]string, map[string]string) {
	stateEnvVars := map[string]string{}
	commandEnvVars := map[string]string{}

	if envs != nil {
		for _, envvar := range envs.State {
			if envvar.Value != "" {
				stateEnvVars[envvar.Name] = envvar.Value
			} else if envvar.ValueFrom != "" {
				stateEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
			}
		}

		for _, envvar := range envs.Commands {
			if envvar.Value != "" {
				commandEnvVars[envvar.Name] = envvar.Value
			} else if envvar.ValueFrom != "" {
				commandEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
			}
		}
	}

	return stateEnvVars, commandEnvVars
}
