package configuration

import (
	"digger/pkg/utils"
	"errors"
	"fmt"
	"github.com/dominikbraun/graph"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"regexp"
)

type DirWalker interface {
	GetDirs(workingDir string) ([]string, error)
}

type FileSystemDirWalker struct {
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

func (walker *FileSystemDirWalker) GetDirs(workingDir string) ([]string, error) {
	var dirs []string
	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}
			if info.IsDir() {
				terraformFiles, _ := GetFilesWithExtension(path, ".tf")
				if len(terraformFiles) > 0 {
					dirs = append(dirs, path)
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

var ErrDiggerConfigConflict = errors.New("more than one digger config file detected, please keep either 'digger.yml' or 'digger.yaml'")

func LoadDiggerConfig(workingDir string, walker DirWalker) (*DiggerConfig, graph.Graph[string, string], error) {
	configYaml := &DiggerConfigYaml{}
	config := &DiggerConfig{}
	fileName, err := retrieveConfigFile(workingDir)
	if err != nil {
		if errors.Is(err, ErrDiggerConfigConflict) {
			return nil, nil, fmt.Errorf("error while retrieving config file: %v", err)
		}
	}

	if fileName == "" {
		fmt.Println("No digger config found, using default one")
		config.Projects = make([]Project, 1)
		project := defaultProject()
		config.Projects[0] = project
		config.Workflows = make(map[string]Workflow)
		config.Workflows["default"] = *defaultWorkflow()
		g := graph.New(graph.StringHash)
		g.AddVertex(project.Name)
		return config, g, nil
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file %s: %v", fileName, err)
	}

	if err := yaml.Unmarshal(data, configYaml); err != nil {
		return nil, nil, fmt.Errorf("error parsing '%s': %v", fileName, err)
	}

	if (configYaml.Projects == nil || len(configYaml.Projects) == 0) && configYaml.GenerateProjectsConfig == nil {
		return nil, nil, fmt.Errorf("no projects configuration found in '%s'", fileName)
	}

	config, projectDependencyGraph, err := ConvertDiggerYamlToConfig(configYaml, workingDir, walker)
	if err != nil {
		return nil, nil, err
	}

	for _, p := range config.Projects {
		_, ok := config.Workflows[p.Workflow]
		if !ok {
			return nil, nil, fmt.Errorf("failed to find workflow config '%s' for project '%s'", p.Workflow, p.Name)
		}
	}
	return config, projectDependencyGraph, nil
}

func defaultProject() Project {
	return Project{
		Name:       "default",
		Dir:        ".",
		Workspace:  "default",
		Terragrunt: false,
		Workflow:   "default",
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

			var maskedValue string
			if envvar.Value != "" {
				stateEnvVars[envvar.Name] = envvar.Value
			} else if envvar.ValueFrom != "" {
				stateEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
			}

			value := stateEnvVars[envvar.Name]
			if len(value) >= 3 {
				maskedValue = value[:3] + "*****"
			}

			fmt.Printf("state env var: %s value: %s\n", envvar.Name, maskedValue)
		}

		for _, envvar := range envs.Commands {
			var maskedValue string

			if envvar.Value != "" {
				commandEnvVars[envvar.Name] = envvar.Value
			} else if envvar.ValueFrom != "" {
				commandEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
			}

			value := stateEnvVars[envvar.Name]
			if len(value) >= 3 {
				maskedValue = value[:3] + "*****"
			}

			fmt.Printf("command env var: %s value %s\n", envvar.Name, maskedValue)
		}
	}

	return stateEnvVars, commandEnvVars
}
