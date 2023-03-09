import json
import logging
import os
import sys
import boto3
from digger_commands import (
    process_new_pull_request,
    process_closed_pull_request,
    process_pull_request_comment,
)
from diggerconfig import digger_config
from githubpr import GitHubPR


logger = logging.getLogger("python_terraform")
logger.setLevel(logging.CRITICAL)


def main(argv):
    print(digger_config)
    dynamodb = boto3.resource("dynamodb")

    token = os.getenv("GITHUB_TOKEN")

    event = os.getenv("CONTEXT_GITHUB")
    j = json.loads(event)
    pr_number = None
    event_name = None
    repo_name = None
    repo_owner = None

    if "repository" in j:
        repo_name = j["repository"]

    if "event_name" in j:
        event_name = j["event_name"]

    if "repository_owner" in j:
        repo_owner = j["repository_owner"]

    print(f"event_name: {event_name}")

    if "pull_request" in j["event"]:
        if "merged" in j["event"]["pull_request"]:
            print(f"pull_request merged: {j['event']['pull_request']['merged']}")
        if "number" in j["event"]["pull_request"]:
            pr_number = j["event"]["pull_request"]["number"]
            print(f"pull_request PR #{pr_number}")
            pull_request = GitHubPR(
                repo_name=repo_name, pull_request=pr_number, github_token=token
            )
            changed_files = pull_request.get_files()
            print("changed files:")
            for cf in changed_files:
                print(cf)

    if "issue" in j["event"] and not pr_number:
        if "number" in j["event"]["issue"]:
            pr_number = j["event"]["issue"]["number"]
            print(f"issue PR #{pr_number}")

    if event_name in ["issue_comment"]:
        print(f"issue comment, pr#: {pr_number}")
        if "event" in j and "comment" in j["event"] and "body" in j["event"]["comment"]:
            comment = j["event"]["comment"]["body"]
            process_pull_request_comment(
                repo_owner,
                repo_name,
                event_name,
                dynamodb,
                pr_number,
                token,
                comment,
            )

    if "action" in j["event"] and event_name == "pull_request":
        if j["event"]["action"] in ["reopened", "opened", "synchronize"]:
            process_new_pull_request(
                repo_owner,
                repo_name,
                event_name,
                dynamodb,
                pr_number,
                token,
            )

        if j["event"]["action"] in ["closed"]:
            process_closed_pull_request(
                repo_owner,
                repo_name,
                event_name,
                dynamodb,
                pr_number,
                token,
            )


if __name__ == "__main__":
    main(sys.argv[1:])
