import re


def parse_project_name(comment):
    match = re.search(r"-p ([a-zA-Z\-]+)", comment)
    if match:
        return match.group(1)
    else:
        return None