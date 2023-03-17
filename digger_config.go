package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
)

type DiggerConfig struct {
	Projects []Project `yaml:"projects"`
}

type Project struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir"`
}

func NewDiggerConfig() (*DiggerConfig, error) {
	config := &DiggerConfig{}
	if data, err := os.ReadFile("digger.yml"); err == nil {
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("error parsing digger.yml: %v", err)
		}
	} else {
		config.Projects = make([]Project, 1)
		config.Projects[0] = Project{Name: "default", Dir: "."}
		return config, nil
	}
	return config, nil
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
			if project.Dir != "" && file[:len(project.Dir)] == project.Dir {
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

type File struct {
	Filename string
}
