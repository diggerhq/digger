import os
import yaml
try:
    from yaml import CLoader as Loader, CDumper as Dumper
except ImportError:
    from yaml import Loader, Dumper

class DiggerConfig():
    def __init__(self):
        if os.path.exists("digger.yml"):
            with open("digger.yml", "r") as f:
                self.config = yaml.load(f, Loader=Loader)
        else:
            self.config = {}

    def get_project(self, project_name):
        for project in self.config["projects"]:
            if project_name == project["name"]:
                return project
        return None
    def get_projects(self, project_name=None):
        if self.config and self.config.get("projects") and not project_name:
            return self.config["projects"]
        elif project_name:
            project = self.get_project(project_name)
            if project:
                return project
        return []

    def get_directory(self, project_name):
        project = self.get_project(project_name)
        return project.get("dir", None)

digger_config = DiggerConfig()

