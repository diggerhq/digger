package configuration

import (
	"errors"
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type WorkflowConfiguration struct {
	OnPullRequestPushed []string `yaml:"on_pull_request_pushed"`
	OnPullRequestClosed []string `yaml:"on_pull_request_closed"`
	OnCommitToDefault   []string `yaml:"on_commit_to_default"`
}

type DiggerConfigYaml struct {
	Projects               []Project               `yaml:"projects"`
	AutoMerge              bool                    `yaml:"auto_merge"`
	Workflows              map[string]Workflow     `yaml:"workflows"`
	CollectUsageData       bool                    `yaml:"collect_usage_data"`
	GenerateProjectsConfig *GenerateProjectsConfig `yaml:"generate_projects"`
}

type EnvVarConfig struct {
	Name      string `yaml:"name"`
	ValueFrom string `yaml:"value_from"`
	Value     string `yaml:"value"`
}

type DiggerConfig struct {
	Projects         []Project
	AutoMerge        bool
	CollectUsageData bool
	Workflows        map[string]Workflow
}

type GenerateProjectsConfig struct {
	Include string `yaml:"include"`
	Exclude string `yaml:"exclude"`
}

type Project struct {
	Name       string `yaml:"name"`
	Dir        string `yaml:"dir"`
	Workspace  string `yaml:"workspace"`
	Terragrunt bool   `yaml:"terragrunt"`
	Workflow   string `yaml:"workflow"`
}

type Stage struct {
	Steps []Step `yaml:"steps"`
}

type Workflow struct {
	EnvVars       EnvVars                `yaml:"env_vars"`
	Plan          *Stage                 `yaml:"plan,omitempty"`
	Apply         *Stage                 `yaml:"apply,omitempty"`
	Configuration *WorkflowConfiguration `yaml:"workflow_configuration"`
}

type EnvVars struct {
	State    []EnvVarConfig `yaml:"state"`
	Commands []EnvVarConfig `yaml:"commands"`
}

type DirWalker interface {
	GetDirs(workingDir string) ([]string, error)
}

type FileSystemDirWalker struct {
}

func (walker *FileSystemDirWalker) GetDirs(workingDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(workingDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return files, nil
}

var ErrDiggerConfigConflict = errors.New("more than one digger config file detected, please keep either 'digger.yml' or 'digger.yaml'")

func (p *Project) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawProject Project
	raw := rawProject{
		Workspace:  "default",
		Terragrunt: false,
		Workflow:   "default",
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*p = Project(raw)
	return nil
}

func (w *Workflow) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawWorkflow Workflow
	raw := rawWorkflow{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
		EnvVars: EnvVars{
			State:    []EnvVarConfig{},
			Commands: []EnvVarConfig{},
		},
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*w = Workflow(raw)
	return nil
}

type Step struct {
	Action    string
	Value     string
	ExtraArgs []string `yaml:"extra_args,omitempty"`
	Shell     string
}

func (s *Step) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		return value.Decode(&s.Action)
	}

	var stepMap map[string]interface{}
	if err := value.Decode(&stepMap); err != nil {
		return err
	}

	if _, ok := stepMap["run"]; ok {
		s.Action = "run"
		s.Value = stepMap["run"].(string)
		if _, ok := stepMap["shell"]; ok {
			s.Shell = stepMap["shell"].(string)
		}
		return nil
	}

	s.extract(stepMap, "plan")
	s.extract(stepMap, "apply")

	return nil
}

func (s *Step) extract(stepMap map[string]interface{}, action string) {
	if _, ok := stepMap[action]; ok {
		s.Action = action
		var extraArgs []string
		if v, ok := stepMap["extra_args"]; ok {
			for _, v := range v.([]interface{}) {
				extraArgs = append(extraArgs, v.(string))
			}
			s.ExtraArgs = extraArgs
		}
	}
}

// duplicate copied from digger.go
func defaultWorkflow() *Workflow {
	return &Workflow{
		Configuration: &WorkflowConfiguration{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &Stage{
			Steps: []Step{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
	}
}

func ConvertDiggerYamlToConfig(diggerYaml *DiggerConfigYaml, workingDir string, walker DirWalker) (*DiggerConfig, error) {
	var diggerConfig DiggerConfig

	diggerConfig.AutoMerge = diggerYaml.AutoMerge

	if diggerYaml.Workflows != nil {
		diggerConfig.Workflows = diggerYaml.Workflows
	} else {
		workflow := *defaultWorkflow()
		diggerConfig.Workflows = make(map[string]Workflow)
		diggerConfig.Workflows["default"] = workflow
	}

	diggerConfig.Projects = diggerYaml.Projects
	diggerConfig.CollectUsageData = diggerYaml.CollectUsageData

	if diggerYaml.GenerateProjectsConfig != nil {
		dirs, err := walker.GetDirs(workingDir)
		if err != nil {
			return nil, err
		}

		for _, dir := range dirs {
			includePattern := diggerYaml.GenerateProjectsConfig.Include
			excludePattern := diggerYaml.GenerateProjectsConfig.Exclude
			includeMatch, err := doublestar.PathMatch(includePattern, dir)
			if err != nil {
				return nil, err
			}

			excludeMatch, err := doublestar.PathMatch(excludePattern, dir)
			if err != nil {
				return nil, err
			}
			if includeMatch && !excludeMatch {
				project := Project{Name: filepath.Base(dir), Dir: filepath.Join(workingDir, dir)}
				diggerConfig.Projects = append(diggerConfig.Projects, project)
			}
		}
	}

	projectNames := make(map[string]bool)

	for _, project := range diggerConfig.Projects {
		if projectNames[project.Name] {
			return nil, fmt.Errorf("project name '%s' is duplicated", project.Name)
		}
		projectNames[project.Name] = true
	}

	return &diggerConfig, nil
}

func NewDiggerConfig(workingDir string, walker DirWalker) (*DiggerConfig, error) {
	config := &DiggerConfigYaml{}
	fileName, err := retrieveConfigFile(workingDir)
	if err != nil {
		if errors.Is(err, ErrDiggerConfigConflict) {
			return nil, fmt.Errorf("error while retrieving config file: %v", err)
		}
	}

	if fileName == "" {
		fmt.Println("No digger config found, using default one")
		config.Projects = make([]Project, 1)
		config.Projects[0] = defaultProject()
		config.Workflows = make(map[string]Workflow)
		config.Workflows["default"] = Workflow{
			Plan: &Stage{
				Steps: []Step{{
					Action:    "init",
					ExtraArgs: []string{},
				}, {
					Action:    "plan",
					ExtraArgs: []string{},
				}},
			},
			Apply: &Stage{
				Steps: []Step{{
					Action:    "init",
					ExtraArgs: []string{},
				}, {
					Action:    "apply",
					ExtraArgs: []string{},
				}},
			},
			Configuration: &WorkflowConfiguration{
				OnPullRequestPushed: []string{"digger plan"},
				OnPullRequestClosed: []string{"digger unlock"},
				OnCommitToDefault:   []string{"digger apply"},
			},
		}
		c, err := ConvertDiggerYamlToConfig(config, workingDir, walker)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %v", fileName, err)
		}
		return c, nil
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", fileName, err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing '%s': %v", fileName, err)
	}

	if (config.Projects == nil || len(config.Projects) == 0) && config.GenerateProjectsConfig == nil {
		return nil, fmt.Errorf("no projects configuration found in '%s'", fileName)
	}

	c, err := ConvertDiggerYamlToConfig(config, workingDir, walker)
	if err != nil {
		return nil, err
	}
	return c, nil
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
		for _, file := range changedFiles {
			absoluteFile, _ := filepath.Abs(path.Join("/", file))
			absoluteDir, _ := filepath.Abs(path.Join("/", project.Dir))

			//fmt.Printf("absoluteFile: %s, absoluteDir: %s \n", absoluteFile, absoluteDir)
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

func (c *DiggerConfig) GetWorkflow(workflowName string) *Workflow {
	workflows := c.Workflows

	workflow, ok := workflows[workflowName]
	if !ok {
		return nil
	}
	return &workflow

}

func (c *DiggerConfig) GetWorkflowConfiguration(projectName string) WorkflowConfiguration {
	project := c.GetProject(projectName)
	workflows := c.Workflows
	if project == nil {
		return WorkflowConfiguration{}
	}
	workflow, ok := workflows[project.Workflow]

	if !ok {
		return WorkflowConfiguration{}
	}
	return *workflow.Configuration
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
