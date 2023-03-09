from simple_lock import get_lock, acquire_lock, release_lock
from tf_utils import (
    cleanup_terraform_plan,
    get_terraform_plan,
    get_terraform_apply,
    cleanup_terraform_apply,
)
from githubpr import GitHubPR
from usage import send_usage_record


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
        # gha_utils.error("Run 'digger apply' to unlock the project.")
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


def digger_apply(
    repo_owner,
    repo_name,
    event_name,
    impacted_projects,
    digger_config,
    dynamodb,
    pr_number,
    token,
):
    send_usage_record(repo_owner, event_name, "apply")
    print(impacted_projects)
    for project in impacted_projects:
        project_name = project["name"]
        lock_id = f"{repo_name}#{project_name}"
        directory = digger_config.get_directory(project_name)
        if lock_project(dynamodb, lock_id, pr_number, token, for_terraform_run=True):
            print("performing apply")
            terraform_apply(dynamodb, lock_id, pr_number, token, directory=directory)
    exit(0)


def digger_plan(
    repo_owner,
    repo_name,
    event_name,
    impacted_projects,
    digger_config,
    dynamodb,
    pr_number,
    token,
):
    send_usage_record(repo_owner, event_name, f"plan")
    for project in impacted_projects:
        project_name = project["name"]
        lock_id = f"{repo_name}#{project_name}"
        directory = digger_config.get_directory(project_name)
        if lock_project(dynamodb, lock_id, pr_number, token, for_terraform_run=True):
            terraform_plan(lock_id, pr_number, token, directory=directory)
    exit(1)


def digger_unlock(
    repo_owner, repo_name, event_name, impacted_projects, dynamodb, pr_number, token
):
    send_usage_record(repo_owner, event_name, "unlock")
    for project in impacted_projects:
        project_name = project["name"]
        lockid = f"{repo_name}#{project_name}"
        force_unlock_project(dynamodb, lockid, pr_number, token)

    # force_unlock_project(dynamodb, lockid, pr_number, token)
