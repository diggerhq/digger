import json
import logging
import os
import sys
import boto3
from digger_commands import (
    digger_apply,
    digger_plan,
    digger_unlock,
    lock_project,
    unlock_project,
)
from utils.io import parse_project_name
from diggerconfig import digger_config
from githubpr import GitHubPR
from usage import send_usage_record


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
            changed_files = pull_request.changed_files()
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
            requested_project = parse_project_name(comment)
            impacted_projects = digger_config.get_projects(requested_project)
            if comment.strip().startswith("digger plan"):
                digger_plan(
                    repo_owner,
                    repo_name,
                    event_name,
                    impacted_projects,
                    digger_config,
                    dynamodb,
                    pr_number,
                    token,
                )
            if comment.strip().startswith("digger apply"):
                digger_apply(
                    repo_owner,
                    repo_name,
                    event_name,
                    impacted_projects,
                    digger_config,
                    dynamodb,
                    pr_number,
                    token,
                )
            if comment.strip().startswith("digger unlock"):
                digger_unlock(
                    repo_owner,
                    repo_name,
                    event_name,
                    impacted_projects,
                    dynamodb,
                    pr_number,
                    token,
                )

    if "action" in j["event"] and event_name == "pull_request":
        if j["event"]["action"] in ["reopened", "opened", "synchronize"]:
            send_usage_record(repo_owner, event_name, "lock")
            lock_acquisition_success = True
            for project in digger_config.get_projects():
                project_name = project["name"]
                lock_id = f"{repo_name}#{project_name}"
                if not lock_project(dynamodb, lock_id, pr_number, token):
                    lock_acquisition_success = False
            if lock_acquisition_success is False:
                exit(1)

        if j["event"]["action"] in ["closed"]:
            send_usage_record(repo_owner, event_name, "unlock")
            for project in digger_config.get_projects():
                project_name = project["name"]
                lock_id = f"{repo_name}#{project_name}"
                unlock_project(dynamodb, lock_id, pr_number, token)


if __name__ == "__main__":
    main(sys.argv[1:])
