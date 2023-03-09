import json
import logging
import os
import sys
import boto3

from code.digger_commands import digger_apply, digger_plan, digger_unlock
from utils.io import parse_project_name
from diggerconfig import digger_config

from githubpr import GitHubPR
from simple_lock import acquire_lock, release_lock, get_lock
from tf_utils import (
    get_terraform_plan,
    get_terraform_apply,
    cleanup_terraform_plan,
    cleanup_terraform_apply,
)
from usage import send_usage_record

import github_action_utils as gha_utils

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


def terraform_plan(lock_id, pr_number, token, directory="."):
    pull_request = GitHubPR(
        repo_name=lock_id, pull_request=pr_number, github_token=token
    )
    return_code, stdout, stderr = get_terraform_plan(directory)
    comment = cleanup_terraform_plan(return_code, stdout, stderr)
    pull_request.publish_comment(f"Plan for **{lock_id}**\n{comment}")


def terraform_apply(dynamodb, lock_id, pr_number, token, directory="."):
    pull_request = GitHubPR(
        repo_name=lock_id, pull_request=pr_number, github_token=token
    )
    ret_code, stdout, stderr = get_terraform_apply(directory)
    comment = cleanup_terraform_apply(ret_code, stdout, stderr)
    pull_request.publish_comment(f"Apply for **{lock_id}**\n{comment}")
    if ret_code == 0 or ret_code == 2:
        unlock_project(dynamodb, lock_id, pr_number, token)


def lock_project(dynamodb, repo_name, pr_number, token, for_terraform_run=False):
    lock = get_lock(dynamodb, repo_name)
    pull_request = GitHubPR(repo_name, pr_number, token)
    print(f"lock_project, lock:{lock}")
    if lock:
        transaction_id = lock["transaction_id"]
        if int(pr_number) != int(transaction_id):
            comment = f"Project locked by another PR #{lock['transaction_id']} (failed to acquire lock {repo_name}). The locking plan must be applied or discarded before future plans can execute"
            pull_request.publish_comment(comment)
            print(comment)
            return False
        else:
            comment = f"Project locked by this PR #{lock['transaction_id']}"
            pull_request.publish_comment(comment)
            print(comment)
            return True

    lock_acquired = acquire_lock(dynamodb, repo_name, 60 * 24, pr_number)
    if lock_acquired:
        comment = f"Project has been locked by PR #{pr_number}"
        pull_request.publish_comment(comment)
        print(f"project locked successfully. PR #{pr_number}")
        gha_utils.error("Run 'digger apply' to unlock the project.")
        # if for_terraform_run:
        #    # if we are going to run terraform we don't need to fail job
        return True
    else:
        lock = get_lock(dynamodb, repo_name)
        comment = f"Project locked by another PR #{lock['transaction_id']} (failed to acquire lock {repo_name}). The locking plan must be applied or discarded before future plans can execute"
        pull_request.publish_comment(comment)
        print(comment)
        return False


def unlock_project(dynamodb, repo_name, pr_number, token):
    lock = get_lock(dynamodb, repo_name)
    if lock:
        print(f"lock: {lock}")
        print(f"pr_number: {pr_number}")
        transaction_id = lock["transaction_id"]
        if int(pr_number) == int(transaction_id):
            lock_released = release_lock(dynamodb, repo_name)
            print(f"lock_released: {lock_released}")
            if lock_released:
                pull_request = GitHubPR(repo_name, pr_number, token)
                comment = f"Project unlocked ({repo_name})."
                pull_request.publish_comment(comment)
                print("Project unlocked")


def force_unlock_project(dynamodb, repo_name, pr_number, token):
    lock = get_lock(dynamodb, repo_name)
    if lock:
        print(f"lock: {lock}")
        lock_released = release_lock(dynamodb, repo_name)
        print(f"lock_released: {lock_released}")
        if lock_released:
            pull_request = GitHubPR(repo_name, pr_number, token)
            comment = f"Project unlocked."
            pull_request.publish_comment(comment)
            print("Project unlocked")


if __name__ == "__main__":
    main(sys.argv[1:])
