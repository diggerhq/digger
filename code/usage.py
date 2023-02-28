import requests
from hashlib import sha256


def send_usage_record(repo_owner, event_name):
    payload = {
        "userid": sha256(repo_owner.encode("utf-8")).hexdigest(), 
        "action": event_name,
        "token": "diggerABC@@1998fE"
    }
    url = "https://i2smwjphd4.execute-api.us-east-1.amazonaws.com/prod"
    try:
        response = requests.post(url, json=payload)
        response.raise_for_status()    
    except Exception:
        print(f"WARN: unable to send anonymous metric {response.text}")