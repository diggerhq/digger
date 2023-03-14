package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type DiggerConfig struct {
	Projects []Project `yaml:"projects"`
}

type Project struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir"`
}

func NewDiggerConfig() *DiggerConfig {
	config := &DiggerConfig{}
	if data, err := ioutil.ReadFile("digger.yml"); err == nil {
		if err := yaml.Unmarshal(data, config); err != nil {
			log.Fatalf("error parsing digger.yml: %v", err)
		}
	} else {
		log.Fatalf("error reading digger.yml: %v", err)
	}
	return config
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

func (c *DiggerConfig) GetModifiedProjects(changedFiles []File) []Project {
	var result []Project
	for _, project := range c.Projects {
		for _, file := range changedFiles {
			if project.Dir != "" && file.Filename[:len(project.Dir)] == project.Dir {
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
