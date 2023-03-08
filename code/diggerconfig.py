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

    def get_projects(self):
        if self.config and self.config.get("projects"):
            return self.config["projects"]
        else:
            return []

    def get_directory(self, project_name):
        return self.config["projects"][project_name]["dir"]

digger_config = DiggerConfig()

