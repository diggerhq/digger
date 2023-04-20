# Workspaces

You can specify a Workspace for a project by using the `workspace` option in `digger.yml`

{% hint style="info" %}
This is about Terraform CLI Workspaces - not Terraform Cloud Workspaces. Those are different things for historic reasons. [Hashicorp article](https://developer.hashicorp.com/terraform/cloud-docs/workspaces#terraform-cloud-vs-terraform-cli-workspaces)
{% endhint %}

So you can have 2 projects linked to the same directory but using different workspaces, like this:

```
projects:
- name: dev
  branch: /main/
  dir: ./
  workspace: dev
  terraform_version: v0.11.0
- name: prod
  branch: /main/
  dir: ./
  workspace: prod
  terraform_version: v0.11.0
```

Example repository: [https://github.com/diggerhq/digger\_demo\_workspaces/](https://github.com/diggerhq/digger\_demo\_workspaces/)
