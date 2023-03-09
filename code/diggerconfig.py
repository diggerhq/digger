import os
import yaml

try:
    from yaml import CLoader as Loader, CDumper as Dumper
except ImportError:
    from yaml import Loader, Dumper


class DiggerConfig:
    def __init__(self):
        if os.path.exists("digger.yml"):
            with open("digger.yml", "r") as f:
                self.config = yaml.load(f, Loader=Loader)
        else:
            self.config = {"projects": {"name": "default", "dir": ""}}

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
                return [project]
        return []

    def get_modified_projects(self, changed_files):
        all_projects = self.get_projects()
        result = []

        for p in all_projects:
            print(f'p: {p}')
            for f in changed_files:
                if "dir" in p and f.filename.startswith(p["dir"]):
                    result.append(p)
        return result

    def get_directory(self, project_name):
        project = self.get_project(project_name)
        return project.get("dir", None)


digger_config = DiggerConfig()
