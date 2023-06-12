package configuration

import (
	"digger/pkg/core/models"
	"digger/pkg/utils"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	CollectUsageData       *bool                   `yaml:"collect_usage_data,omitempty"`
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
	Name            string   `yaml:"name"`
	Dir             string   `yaml:"dir"`
	Workspace       string   `yaml:"workspace"`
	Terragrunt      bool     `yaml:"terragrunt"`
	Workflow        string   `yaml:"workflow"`
	IncludePatterns []string `yaml:"include_patterns,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`
}

type Stage struct {
	Steps []Step `yaml:"steps"`
}

func (s *Stage) ToCoreStage() models.Stage {
	var steps []models.Step
	for _, step := range s.Steps {
		steps = append(steps, step.ToCoreStep())
	}
	return models.Stage{Steps: steps}
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

func (s *Step) ToCoreStep() models.Step {
	return models.Step{
		Action:    s.Action,
		Value:     s.Value,
		ExtraArgs: s.ExtraArgs,
		Shell:     s.Shell,
	}
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
		if v, ok := stepMap[action].(map[string]interface{})["extra_args"]; ok {
			for _, v := range v.([]interface{}) {
				extraArgs = append(extraArgs, v.(string))
			}
			s.ExtraArgs = extraArgs
		}
	}
}

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
	const defaultWorkflowName = "default"

	diggerConfig.AutoMerge = diggerYaml.AutoMerge

	// if workflow block is not specified in yaml we create a default one, and add it to every project
	if diggerYaml.Workflows != nil {
		diggerConfig.Workflows = diggerYaml.Workflows
		// provide default workflow if not specified
		if _, ok := diggerConfig.Workflows[defaultWorkflowName]; !ok {
			workflow := *defaultWorkflow()
			diggerConfig.Workflows[defaultWorkflowName] = workflow
		}
	} else {
		workflow := *defaultWorkflow()
		diggerConfig.Workflows = make(map[string]Workflow)
		diggerConfig.Workflows[defaultWorkflowName] = workflow
	}

	diggerConfig.Projects = diggerYaml.Projects

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
	if diggerYaml.CollectUsageData != nil {
		diggerConfig.CollectUsageData = *diggerYaml.CollectUsageData
	} else {
		diggerConfig.CollectUsageData = true
	}

	if diggerYaml.GenerateProjectsConfig != nil {
		dirs, err := walker.GetDirs(workingDir)
		if err != nil {
			return nil, err
		}

		for _, dir := range dirs {
			includePattern := diggerYaml.GenerateProjectsConfig.Include
			excludePattern := diggerYaml.GenerateProjectsConfig.Exclude
			if utils.MatchIncludeExcludePatternsToFile(dir, []string{includePattern}, []string{excludePattern}) {
				project := Project{Name: filepath.Base(dir), Dir: dir, Workflow: defaultWorkflowName, Workspace: "default"}
				diggerConfig.Projects = append(diggerConfig.Projects, project)
			}
		}
	}
	return &diggerConfig, nil
}

func LoadDiggerConfig(workingDir string, walker DirWalker) (*DiggerConfig, error) {
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

	for _, p := range c.Projects {
		_, ok := c.Workflows[p.Workflow]
		if !ok {
			return nil, fmt.Errorf("failed to find workflow config '%s' for project '%s'", p.Workflow, p.Name)
		}
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

func CollectEnvVars(envs EnvVars) (map[string]string, map[string]string) {
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
