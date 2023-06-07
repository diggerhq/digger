package configuration

import (
	"digger/pkg/utils"
	"errors"
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/jinzhu/copier"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
)

type DiggerConfigYaml struct {
	Projects               []ProjectYaml               `yaml:"projects"`
	AutoMerge              bool                        `yaml:"auto_merge"`
	Workflows              map[string]WorkflowYaml     `yaml:"workflows"`
	CollectUsageData       bool                        `yaml:"collect_usage_data"`
	GenerateProjectsConfig *GenerateProjectsConfigYaml `yaml:"generate_projects"`
}

type ProjectYaml struct {
	Name            string   `yaml:"name"`
	Dir             string   `yaml:"dir"`
	Workspace       string   `yaml:"workspace"`
	Terragrunt      bool     `yaml:"terragrunt"`
	Workflow        string   `yaml:"workflow"`
	IncludePatterns []string `yaml:"include_patterns,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`
}

type WorkflowYaml struct {
	EnvVars       *EnvVarsYaml               `yaml:"env_vars"`
	Plan          *StageYaml                 `yaml:"plan,omitempty"`
	Apply         *StageYaml                 `yaml:"apply,omitempty"`
	Configuration *WorkflowConfigurationYaml `yaml:"workflow_configuration"`
}

type WorkflowConfigurationYaml struct {
	OnPullRequestPushed []string `yaml:"on_pull_request_pushed"`
	OnPullRequestClosed []string `yaml:"on_pull_request_closed"`
	OnCommitToDefault   []string `yaml:"on_commit_to_default"`
}

type StageYaml struct {
	Steps []StepYaml `yaml:"steps"`
}

type StepYaml struct {
	Action    string
	Value     string
	ExtraArgs []string `yaml:"extra_args,omitempty"`
	Shell     string
}

type EnvVarsYaml struct {
	State    []EnvVarConfigYaml `yaml:"state"`
	Commands []EnvVarConfigYaml `yaml:"commands"`
}

type EnvVarConfigYaml struct {
	Name      string `yaml:"name"`
	ValueFrom string `yaml:"value_from"`
	Value     string `yaml:"value"`
}

type GenerateProjectsConfigYaml struct {
	Include string `yaml:"include"`
	Exclude string `yaml:"exclude"`
}

type DiggerConfig struct {
	Projects         []ProjectConfig
	AutoMerge        bool
	CollectUsageData bool
	Workflows        map[string]WorkflowConfig
}

type ProjectConfig struct {
	Name            string
	Dir             string
	Workspace       string
	Terragrunt      bool
	Workflow        string
	IncludePatterns []string
	ExcludePatterns []string
}

type WorkflowConfig struct {
	EnvVars       *EnvVarsConfig
	Plan          *StageConfig
	Apply         *StageConfig
	Configuration *WorkflowConfigurationConfig
}

type WorkflowConfigurationConfig struct {
	OnPullRequestPushed []string
	OnPullRequestClosed []string
	OnCommitToDefault   []string
}

type StageConfig struct {
	Steps []StepConfig
}

type StepConfig struct {
	Action    string
	Value     string
	ExtraArgs []string
	Shell     string
}

type EnvVarsConfig struct {
	State    []EnvVarConfigConfig
	Commands []EnvVarConfigConfig
}

type EnvVarConfigConfig struct {
	Name      string
	ValueFrom string
	Value     string
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

func (p *ProjectYaml) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawProject ProjectYaml
	raw := rawProject{
		Workspace:  "default",
		Terragrunt: false,
		Workflow:   "default",
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*p = ProjectYaml(raw)
	return nil
}

func (w *WorkflowYaml) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawWorkflow WorkflowYaml
	raw := rawWorkflow{
		Configuration: &WorkflowConfigurationYaml{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &StageYaml{
			Steps: []StepYaml{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &StageYaml{
			Steps: []StepYaml{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "apply", ExtraArgs: []string{},
				},
			},
		},
		EnvVars: &EnvVarsYaml{
			State:    []EnvVarConfigYaml{},
			Commands: []EnvVarConfigYaml{},
		},
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*w = WorkflowYaml(raw)
	return nil
}

func (s *StepYaml) UnmarshalYAML(value *yaml.Node) error {
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

func (s *StepYaml) extract(stepMap map[string]interface{}, action string) {
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

func defaultWorkflow() *WorkflowConfig {
	return &WorkflowConfig{
		Configuration: &WorkflowConfigurationConfig{
			OnCommitToDefault:   []string{"digger unlock"},
			OnPullRequestPushed: []string{"digger plan"},
			OnPullRequestClosed: []string{"digger unlock"},
		},
		Plan: &StageConfig{
			Steps: []StepConfig{
				{
					Action: "init", ExtraArgs: []string{},
				},
				{
					Action: "plan", ExtraArgs: []string{},
				},
			},
		},
		Apply: &StageConfig{
			Steps: []StepConfig{
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

func copyProjects(projects []ProjectYaml) []ProjectConfig {
	result := make([]ProjectConfig, len(projects))
	for i, p := range projects {
		item := ProjectConfig{p.Name,
			p.Dir,
			p.Workspace,
			p.Terragrunt,
			p.Workflow,
			p.IncludePatterns,
			p.ExcludePatterns,
		}
		result[i] = item
	}
	return result
}

func copyEnvVars(envVars *EnvVarsYaml) *EnvVarsConfig {
	result := EnvVarsConfig{}
	result.State = make([]EnvVarConfigConfig, len(envVars.State))
	result.Commands = make([]EnvVarConfigConfig, len(envVars.Commands))

	for i, s := range envVars.State {
		item := EnvVarConfigConfig{
			s.Name,
			s.ValueFrom,
			s.Value,
		}
		result.State[i] = item
	}
	for i, s := range envVars.Commands {
		item := EnvVarConfigConfig{
			s.Name,
			s.ValueFrom,
			s.Value,
		}
		result.Commands[i] = item
	}

	return &result
}

func copyStage(stage *StageYaml) *StageConfig {
	result := StageConfig{}
	result.Steps = make([]StepConfig, len(stage.Steps))

	for i, s := range stage.Steps {
		item := StepConfig{
			s.Action,
			s.Value,
			s.ExtraArgs,
			s.Shell,
		}
		result.Steps[i] = item
	}
	return &result
}

func copyWorkflowConfiguration(config *WorkflowConfigurationYaml) *WorkflowConfigurationConfig {
	result := WorkflowConfigurationConfig{}
	result.OnPullRequestClosed = make([]string, len(config.OnPullRequestClosed))
	result.OnPullRequestPushed = make([]string, len(config.OnPullRequestPushed))
	result.OnCommitToDefault = make([]string, len(config.OnCommitToDefault))

	result.OnPullRequestClosed = config.OnPullRequestClosed
	result.OnPullRequestPushed = config.OnPullRequestPushed
	result.OnCommitToDefault = config.OnCommitToDefault
	return &result
}

func copyWorkflows(workflows map[string]WorkflowYaml) map[string]WorkflowConfig {
	result := make(map[string]WorkflowConfig, len(workflows))
	for i, w := range workflows {
		envVars := copyEnvVars(w.EnvVars)
		plan := copyStage(w.Plan)
		apply := copyStage(w.Apply)
		configuration := copyWorkflowConfiguration(w.Configuration)
		item := WorkflowConfig{
			envVars,
			plan,
			apply,
			configuration,
		}
		copier.Copy(&w, &item)
		result[i] = item
	}
	return result
}

func ConvertDiggerYamlToConfig(diggerYaml *DiggerConfigYaml, workingDir string, walker DirWalker) (*DiggerConfig, error) {
	var diggerConfig DiggerConfig
	const defaultWorkflowName = "default"

	diggerConfig.AutoMerge = diggerYaml.AutoMerge

	// if workflow block is not specified in yaml we create a default one, and add it to every project
	if diggerYaml.Workflows != nil {
		workflows := copyWorkflows(diggerYaml.Workflows)
		diggerConfig.Workflows = workflows

	} else {
		workflow := *defaultWorkflow()
		diggerConfig.Workflows = make(map[string]WorkflowConfig)
		diggerConfig.Workflows[defaultWorkflowName] = workflow
	}

	projects := copyProjects(diggerYaml.Projects)
	diggerConfig.Projects = projects

	// update project's workflow if needed
	for _, project := range diggerConfig.Projects {
		if project.Workflow == "" {
			project.Workflow = defaultWorkflowName
		}
	}

	// check for project name duplicates
	projectNames := make(map[string]bool)
	for _, project := range diggerConfig.Projects {
		if projectNames[project.Name] {
			return nil, fmt.Errorf("project name '%s' is duplicated", project.Name)
		}
		projectNames[project.Name] = true
	}

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
				// generate a new project using default workflow
				project := ProjectConfig{Name: filepath.Base(dir), Dir: filepath.Join(workingDir, dir), Workflow: defaultWorkflowName}
				diggerConfig.Projects = append(diggerConfig.Projects, project)
			}
		}
	}
	return &diggerConfig, nil
}

func LoadDiggerConfig(workingDir string, walker DirWalker) (*DiggerConfig, error) {
	configYaml := &DiggerConfigYaml{}
	config := &DiggerConfig{}
	fileName, err := retrieveConfigFile(workingDir)
	if err != nil {
		if errors.Is(err, ErrDiggerConfigConflict) {
			return nil, fmt.Errorf("error while retrieving config file: %v", err)
		}
	}

	if fileName == "" {
		fmt.Println("No digger config found, using default one")
		config.Projects = make([]ProjectConfig, 1)
		config.Projects[0] = defaultProject()
		config.Workflows = make(map[string]WorkflowConfig)
		config.Workflows["default"] = *defaultWorkflow()
		return config, nil
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", fileName, err)
	}

	if err := yaml.Unmarshal(data, configYaml); err != nil {
		return nil, fmt.Errorf("error parsing '%s': %v", fileName, err)
	}

	if (configYaml.Projects == nil || len(configYaml.Projects) == 0) && configYaml.GenerateProjectsConfig == nil {
		return nil, fmt.Errorf("no projects configuration found in '%s'", fileName)
	}

	c, err := ConvertDiggerYamlToConfig(configYaml, workingDir, walker)
	if err != nil {
		return nil, err
	}

	for _, p := range c.Projects {
		_, ok := c.Workflows[p.Workflow]
		if !ok {
			return nil, fmt.Errorf("failed to find workflow config '%s' for project '%s'", p.Workflow, p.Name)
		}
	}
	return c, nil
}

func defaultProject() ProjectConfig {
	return ProjectConfig{
		Name:       "default",
		Dir:        ".",
		Workspace:  "default",
		Terragrunt: false,
		Workflow:   "default",
	}
}

func (c *DiggerConfig) GetProject(projectName string) *ProjectConfig {
	for _, project := range c.Projects {
		if projectName == project.Name {
			return &project
		}
	}
	return nil
}

func (c *DiggerConfig) GetProjects(projectName string) []ProjectConfig {
	if projectName == "" {
		return c.Projects
	}
	project := c.GetProject(projectName)
	if project == nil {
		return nil
	}
	return []ProjectConfig{*project}
}

func (c *DiggerConfig) GetModifiedProjects(changedFiles []string) []ProjectConfig {
	var result []ProjectConfig
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

func (c *DiggerConfig) GetWorkflow(workflowName string) *WorkflowConfig {
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

func CollectEnvVars(envs *EnvVarsConfig) (map[string]string, map[string]string) {
	stateEnvVars := map[string]string{}

	for _, envvar := range envs.State {
		if envvar.Value != "" {
			stateEnvVars[envvar.Name] = envvar.Value
		} else if envvar.ValueFrom != "" {
			stateEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
		}
	}

	commandEnvVars := map[string]string{}

	for _, envvar := range envs.Commands {
		if envvar.Value != "" {
			commandEnvVars[envvar.Name] = envvar.Value
		} else if envvar.ValueFrom != "" {
			commandEnvVars[envvar.Name] = os.Getenv(envvar.ValueFrom)
		}
	}
	return stateEnvVars, commandEnvVars
}
