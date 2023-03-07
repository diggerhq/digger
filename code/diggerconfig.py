import os
import yaml
try:
    from yaml import CLoader as Loader, CDumper as Dumper
except ImportError:
    from yaml import Loader, Dumper


def load_digger_config():
    print("loading digger config !!!!")
    if os.path.exists("digger.yml"):
        with open("digger.yml", "r") as f:
            return yaml.load(f, Loader=Loader)
    else:
        return {}

digger_config = load_digger_config()
