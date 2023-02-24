import json
import os
import sys
import boto3

from githubpr import GitHubPR
from simple_lock import acquire_lock, release_lock, get_lock, create_locks_table_if_not_exists
from terraform_plan_test import get_terraform_plan, get_terraform_apply
import github_action_utils as gha_utils


def main(argv):
    dynamodb = boto3.resource("dynamodb")
    create_locks_table_if_not_exists(dynamodb)

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
    if "repository" in j:
        repo_name = j["repository"]

    if "event_name" in j:
        event_name = j["event_name"]

    print(f"event_name: {event_name}")

    if (
        event_name not in ["issue_comment"]
        and ref_name
        and not head_ref
        and not base_ref
    ):
        print(f"commit merged to {ref_name}")
        # lock_released = release_lock(dynamodb, "test_github_actions", "tx-1")
        # if lock_released:
        #    print("Project unlocked")

    if "pull_request" in j["event"]:
        if "merged" in j["event"]["pull_request"]:
            print(f"pull_request merged: {j['event']['pull_request']['merged']}")
        if "number" in j["event"]["pull_request"]:
            pr_number = j["event"]["pull_request"]["number"]
            print(f"pull_request pr# {pr_number}")

    if "issue" in j["event"] and not pr_number:
        if "number" in j["event"]["issue"]:
            pr_number = j["event"]["issue"]["number"]
            print(f"issue pr# {pr_number}")

    if event_name in ["issue_comment"]:
        print(f"issue comment, pr#: {pr_number}")
        if "event" in j and "comment" in j["event"] and "body" in j["event"]["comment"]:
            comment = j["event"]["comment"]["body"]
            if comment.strip() == "digger plan":
                terraform_plan(dynamodb, repo_name, pr_number, token)

            if comment.strip() == "digger apply":
                terraform_apply(dynamodb, repo_name, pr_number, token)

    if "action" in j["event"] and event_name == "pull_request":
        if j["event"]["action"] in ["reopened", "opened", "synchronize"]:
            print("Pull request opened.")
            lock_project(dynamodb, repo_name, pr_number, token)

        if j["event"]["action"] in ["closed"]:
            print("Pull request closed.")
            unlock_project(dynamodb, repo_name, pr_number, token)


def terraform_plan(dynamodb, repo_name, pr_number, token):
    lock_project(dynamodb, repo_name, pr_number, token, for_terraform_run=True)
    pull_request = GitHubPR(repo_name, pr_number, token)
    comment = get_terraform_plan()
    pull_request.publish_comment(comment)


def terraform_apply(dynamodb, repo_name, pr_number, token):
    lock_project(dynamodb, repo_name, pr_number, token, for_terraform_run=True)
    pull_request = GitHubPR(repo_name, pr_number, token)
    ret_code, comment = get_terraform_apply()
    pull_request.publish_comment(comment)
    if ret_code == 0 or ret_code == 2:
        unlock_project(dynamodb, repo_name, pr_number, token)


def lock_project(dynamodb, repo_name, pr_number, token, for_terraform_run=False):
    lock = get_lock(dynamodb, repo_name)
    pull_request = GitHubPR(repo_name, pr_number, token)

    if lock:
        transaction_id = lock["transaction_id"]
        if int(pr_number) != int(transaction_id):
            comment = f"Project locked by another PR# {lock['transaction_id']}"
            pull_request.publish_comment(comment)
            print(comment)
            exit(1)
        else:
            comment = f"Project locked by this PR# {lock['transaction_id']}"
            pull_request.publish_comment(comment)
            print(comment)
            return

    lock_acquired = acquire_lock(dynamodb, repo_name, 60 * 24, pr_number)
    if lock_acquired:
        comment = f"Project has been locked by PR# {pr_number}"
        pull_request.publish_comment(comment)
        print(f"project locked successfully. pr#: {pr_number}")
        gha_utils.error("Run 'digger apply' to unlock the project.")
        # if for_terraform_run:
        #    # if we are going to run terraform we don't need to fail job
        #    return
    else:
        lock = get_lock(dynamodb, repo_name)
        comment = f"Project locked by another PR# {lock['transaction_id']}"
        pull_request.publish_comment(comment)
        print(comment)


def unlock_project(dynamodb, repo_name, pr_number, token):
    lock = get_lock(dynamodb, repo_name)
    if lock:
        print(f"lock: {lock}")
        print(f"pr_number: {pr_number}")
        transaction_id = lock["transaction_id"]
        if int(pr_number) == int(transaction_id):
            lock_released = release_lock(dynamodb, repo_name, "tx-1")
            print(f"lock_released: {lock_released}")
            if lock_released:
                pull_request = GitHubPR(repo_name, pr_number, token)
                comment = f"Project unlocked."
                pull_request.publish_comment(comment)
                print("Project unlocked")


if __name__ == "__main__":
    main(sys.argv[1:])


"""


if ref_name and not head_ref and not base_ref:
    print(f"PR merged to {ref_name}")
elif ref_name and head_ref and base_ref:
    print(f"PR open from {head_ref} to {base_ref}")

print(f"GITHUB_REF_NAME: {os.getenv('GITHUB_REF_NAME')}")
print(f"GITHUB_BASE_REF: {os.getenv('GITHUB_BASE_REF')}")
print(f"GITHUB_HEAD_REF: {os.getenv('GITHUB_HEAD_REF')}")
print(f"GITHUB_REF_TYPE: {os.getenv('GITHUB_REF_TYPE')}")

"""
