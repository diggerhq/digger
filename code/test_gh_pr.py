import json
import os

from githubpr import GitHubPR

from terraform_plan_test import get_terraform_plan
event = os.getenv("CONTEXT_GITHUB")
j = json.loads(event)

token = os.getenv("GITHUB_TOKEN")

if 'pull_request' in j['event']:
    if 'merged' in j['event']['pull_request']:
        print(f"pull_request merged: {j['event']['pull_request']['merged']}")
    if 'number' in j['event']['pull_request']:
        pr_number = int(j['event']['pull_request']['number'])
        pull_request = GitHubPR('diggerhq/test_github_actions', pr_number, token)
        comment = get_terraform_plan()
        result = pull_request.publish_comment(comment)


