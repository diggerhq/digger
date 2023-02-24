from python_terraform import *


def get_terraform_plan():
    t = Terraform()
    return_code, stdout, stderr = t.init()
    output = stdout
    return_code, stdout, stderr = t.plan()
    output += stdout
    return stdout


def get_terraform_apply():
    t = Terraform()

    return_code, stdout, stderr = t.init()
    output = stdout
    return_code, stdout, stderr = t.plan()
    output += stdout
    return_code, stdout, stderr = t.apply(skip_plan=True)
    output += stdout
    return return_code, output
