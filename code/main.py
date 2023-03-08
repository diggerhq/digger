import json
import logging
import os
import sys
import boto3

from code.io import parse_project_name
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

    base_ref = os.getenv("GITHUB_BASE_REF")
    head_ref = os.getenv("GITHUB_HEAD_REF")
    ref_name = os.getenv("GITHUB_REF_NAME")
    ref_type = os.getenv("GITHUB_REF_TYPE")
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

    if "issue" in j["event"] and not pr_number:
        if "number" in j["event"]["issue"]:
            pr_number = j["event"]["issue"]["number"]
            print(f"issue PR #{pr_number}")

    if event_name in ["issue_comment"]:
        print(f"issue comment, pr#: {pr_number}")
        if "event" in j and "comment" in j["event"] and "body" in j["event"]["comment"]:
            comment = j["event"]["comment"]["body"]
            requested_project = parse_project_name(comment)
            lockid = f"{repo_name}#{requested_project}"
            directory = digger_config.get_directory(requested_project)
            if comment.strip().startswith("digger plan"):
                send_usage_record(repo_owner, event_name, f"plan")
                if lock_project(dynamodb, lockid, pr_number, token, for_terraform_run=True):
                    terraform_plan(dynamodb, lockid, pr_number, token, directory=directory)
                else:
                    exit(1)
            if comment.strip().startswith("digger apply"):
                send_usage_record(repo_owner, event_name, "apply")
                if lock_project(dynamodb, lockid, pr_number, token, for_terraform_run=True):
                    terraform_apply(dynamodb, lockid, pr_number, token, directory=directory)
                else:
                    exit(1)
            if comment.strip().startswith("digger unlock"):
                send_usage_record(repo_owner, event_name, "unlock")
                force_unlock_project(dynamodb, lockid, pr_number, token)

    if "action" in j["event"] and event_name == "pull_request":
        lock_aquisition_success = True
        for project in digger_config.config["projects"]:
            project_name = project["name"]
            lockid = f"{repo_name}#{project_name}"
            if j["event"]["action"] in ["reopened", "opened", "synchronize"]:
                
                send_usage_record(repo_owner, event_name, "lock")
                if not lock_project(dynamodb, lockid, pr_number, token):
                    lock_aquisition_success = False

            if j["event"]["action"] in ["closed"]:
                
                send_usage_record(repo_owner, event_name, "unlock")
                unlock_project(dynamodb, lockid, pr_number, token)

        if lock_aquisition_success is False:
            exit(1)


def terraform_plan(dynamodb, repo_name, pr_number, token, directory="."):
    pull_request = GitHubPR(repo_name, pr_number, token)
    return_code, stdout, stderr = get_terraform_plan(directory)
    comment = cleanup_terraform_plan(return_code, stdout, stderr)
    pull_request.publish_comment(comment)


def terraform_apply(dynamodb, repo_name, pr_number, token, directory="."):
    pull_request = GitHubPR(repo_name, pr_number, token)
    ret_code, stdout, stderr = get_terraform_apply(directory)
    comment = cleanup_terraform_apply(ret_code, stdout, stderr)
    pull_request.publish_comment(comment)
    if ret_code == 0 or ret_code == 2:
        unlock_project(dynamodb, repo_name, pr_number, token)


def lock_project(dynamodb, repo_name, pr_number, token, for_terraform_run=False):
    lock = get_lock(dynamodb, repo_name)
    pull_request = GitHubPR(repo_name, pr_number, token)
    print(f"lock_project, lock:{lock}")
    if lock:
        transaction_id = lock["transaction_id"]
        if int(pr_number) != int(transaction_id):
            comment = f"Project locked by another PR #{lock['transaction_id']} (failed to acquire lock). The locking plan must be applied or discarded before future plans can execute"
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
        comment = f"Project locked by another PR #{lock['transaction_id']} (failed to acquire lock). The locking plan must be applied or discarded before future plans can execute"
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
                comment = f"Project unlocked."
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
