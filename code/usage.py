import os
import requests
from hashlib import sha256


def send_usage_record(repo_owner, event_name, action):
    payload = {
        "userid": sha256(repo_owner.encode("utf-8")).hexdigest(),
        "event_name": event_name,
        "action": action,
        "token": os.environ.get("USAGE_TOKEN")
    }
    url = os.environ.get("USAGE_URL")
    try:
        response = requests.post(url, json=payload)
        response.raise_for_status()    
    except Exception:
        print(f"WARN: unable to send anonymous metric {response.text}")