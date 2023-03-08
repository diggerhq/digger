import os
import yaml
try:
    from yaml import CLoader as Loader, CDumper as Dumper
except ImportError:
    from yaml import Loader, Dumper

class DiggerConfig():
    def __init__(self):
        print("loading digger config !!!!")
        if os.path.exists("digger.yml"):
            with open("digger.yml", "r") as f:
                self.config = yaml.load(f, Loader=Loader)
        else:
            self.config = {}

    def get_projects(self, project_name=None):
        if self.config and self.config.get("projects") and project_name is None:
            return self.config["projects"]
        else:
            return []

    def get_directory(self, project_name):
        for project in  self.config["projects"]:
            if project_name == project["name"]:
                return project["dir"]
        return None

digger_config = DiggerConfig()

