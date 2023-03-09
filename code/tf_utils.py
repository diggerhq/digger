import re

from python_terraform import *


def cleanup_terraform_plan(return_code: int, stdout: str, stderr: str):
    if return_code == 1:
        if stdout:
            error = stdout
        if stderr:
            error = stderr
        return "```terraform\n" + error + "\n```"

    result = None

    if return_code == 0:
        # Succeeded, with empty diff (no changes)
        start = "No changes. Your infrastructure matches the configuration."

    if return_code == 2:
        # Succeeded, with non-empty diff (changes present)
        start = "Terraform will perform the following actions:"
    end_pos = len(stdout)

    try:
        start_pos = stdout.index(start)
    except ValueError:
        start_pos = 0

    regex = r"(Plan: [0-9]+ to add, [0-9]+ to change, [0-9]+ to destroy.)"
    matches = re.search(regex, stdout, re.MULTILINE)
    if matches:
        end_pos = matches.end()

    result = stdout[start_pos:end_pos]

    return "```terraform\n" + result + "\n```"


def cleanup_terraform_apply(return_code: int, stdout: str, stderr: str):
    if return_code == 1:
        return "```terraform\n" + stderr + "\n```"
    start = ""
    if return_code == 0:
        # Succeeded, with empty diff (no changes)
        start = "No changes. Your infrastructure matches the configuration."

    if return_code == 2:
        # Succeeded, with non-empty diff (changes present)
        start = "Terraform will perform the following actions:"
    end_pos = len(stdout)

    try:
        start_pos = stdout.index(start)
    except ValueError:
        start_pos = 0

    regex = (
        r"(Apply complete! Resources: [0-9]+ added, [0-9]+ changed, [0-9]+ destroyed.)"
    )
    matches = re.search(regex, stdout, re.MULTILINE)
    if matches:
        end_pos = matches.end()

    result = stdout[start_pos:end_pos]

    return "```terraform\n" + result + "\n```"


def get_terraform_plan(directory):
    t = Terraform(working_dir=directory)
    r = t.version()
    print(r)
    return_code, stdout, stderr = t.init()
    return_code, stdout, stderr = t.plan()
    return return_code, stdout, stderr


def get_terraform_apply(directory):
    t = Terraform(working_dir=directory)
    return_code, stdout, stderr = t.init()
    return_code, stdout, stderr = t.apply(skip_plan=True)
    return return_code, stdout, stderr
