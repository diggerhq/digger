# digger.yml

You can configure Digger by dropping a `digger.yml` file at the root level of your repo

```
projects:
- name: my-first-app
  dir: app-one
- name: my-second-app
  dir: app-two
```

## Projects

A project in Digger corresponds to a directory containing Terraform code. Projects  are treated as standalone independent entities with their own locks. Digger will not prevent you from running plan and apply in different projects simultaneously.

You can run plan / apply in a specified project by using the -p option in Github PR comment:

```
digger apply -p my-second-app
```
