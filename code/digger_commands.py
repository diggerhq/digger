from code.main import (
    lock_project,
    terraform_apply,
    terraform_plan,
    force_unlock_project,
)
from code.usage import send_usage_record


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
