package digger

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type WorkflowConfiguration struct {
	OnPullRequestPushed []string `yaml:"on_pull_request_pushed"`
	OnPullRequestClosed []string `yaml:"on_pull_request_closed"`
	OnCommitToDefault   []string `yaml:"on_commit_to_default"`
}

type DiggerConfig struct {
	Projects []Project `yaml:"projects"`
}

type Project struct {
	Name                  string                `yaml:"name"`
	Dir                   string                `yaml:"dir"`
	Workspace             string                `yaml:"workspace"`
	WorkflowConfiguration WorkflowConfiguration `yaml:"workflow_configuration"`
}

var ErrDiggerConfigConflict = errors.New("more than one digger config file detected, please keep either 'digger.yml' or 'digger.yaml'")

func (p *Project) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawProject Project
	raw := rawProject{
		Workspace: "default",
		WorkflowConfiguration: WorkflowConfiguration{
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
			OnCommitToDefault:   []string{"digger apply"},
		},
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*p = Project(raw)
	return nil

}

func NewDiggerConfig(workingDir string) (*DiggerConfig, error) {
	config := &DiggerConfig{}
	fileName, err := retrieveConfigFile(workingDir)
	if err != nil {
		if errors.Is(err, ErrDiggerConfigConflict) {
			return nil, fmt.Errorf("error while retrieving config file: %v", err)
		}
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		config.Projects = make([]Project, 1)
		config.Projects[0] = defaultProject()
		return config, nil
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing '%s': %v", fileName, err)
	}

	return config, nil
}

func defaultProject() Project {
	return Project{
		Name:      "default",
		Dir:       ".",
		Workspace: "default",
		WorkflowConfiguration: WorkflowConfiguration{
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
			OnCommitToDefault:   []string{"digger apply"},
		}}
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
		for _, file := range changedFiles {
			absoluteFile, _ := filepath.Abs(path.Join("/", file))
			absoluteDir, _ := filepath.Abs(path.Join("/", project.Dir))
			if strings.HasPrefix(absoluteFile, absoluteDir) {
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

func (c *DiggerConfig) GetWorkflowConfiguration(projectName string) WorkflowConfiguration {
	project := c.GetProject(projectName)
	if project == nil {
		return WorkflowConfiguration{}
	}
	return project.WorkflowConfiguration
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
		return "digger.yml", nil
	}
	if yamlCfg {
		return "digger.yaml", nil
	}

	// Passing this point means digger config file is
	// missing which is a non-error
	return "", nil
}
