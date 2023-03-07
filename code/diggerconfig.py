import os
import yaml

def load_digger_config():
    print("loading digger config !!!!")
    if os.path.exists("digger.yml"):
        with open("digger.yml", "r") as f:
            return yaml.load(f)
    else:
        return {}

digger_config = load_digger_config()
