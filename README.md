# tfrun by Digger

<img width="733" alt="Screenshot 2023-02-28 at 11 25 48" src="https://user-images.githubusercontent.com/1280498/221849642-ae6cb056-5b5b-478f-8cfb-42790e1739e7.png">

TFrun is Github Action that runs Terraform `plan` and `apply` with PR-level locks
Unlike TF Cloud / Spacelift, terraform jobs run natively in your Github Actions - no need to share sensitive data with another CI system
Unlike Atlantis, there's no need to deploy and maintain a backend service.

<img width="693" alt="Screenshot 2023-02-24 at 19 52 12" src="https://user-images.githubusercontent.com/1280498/221277610-368ae950-6319-4bf3-9df2-ca75ca5a05f9.png">

## Features
- code-level locks - only 1 open PR can run plan / apply. This avoids conflicts
- no need to install any backend into your infra - locks are stored in DynamoDB

## How to use

This is demo flow with a sample repo using local state - for real world scenario you'll need to configure remote backend (S3 + DynamoDB) and add a [workflow file](https://github.com/diggerhq/tfrun_demo/blob/main/.github/workflows/plan.yml) to the root of the repo.

1. Fork the [demo repository](https://github.com/diggerhq/tfrun_demo)
2. Enable Actions (by default workflows won't trigger in a fork)

<img width="1441" alt="Screenshot 2023-02-24 at 20 24 08" src="https://user-images.githubusercontent.com/1280498/221291130-6831d45a-008f-452f-91d3-37ba133d7cbb.png">

2. Add environment variables into your Github Action Secrets
- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY
3. make a change and create a PR - this will create a lock
4. comment `digger plan` - terraform plan output will be added as comment. If you don't see a comment (bug) - check out job output
5. create another PR - plan or apply won’t work in this PR until the first lock is released
6. you should see `Locked by PR #1` comment. The action logs will display "Project locked" error message.

## Remote backend and state-level locks

tfrun does not interfere with your remote backend setup. You could be using [S3 backend](https://developer.hashicorp.com/terraform/language/settings/backends/s3) or TF cloud's [remote backend](https://developer.hashicorp.com/terraform/language/settings/backends/remote) or [some other way](https://developer.hashicorp.com/terraform/language/settings/backends/configuration)

tfrun also doesn't differentiate locks based on statefiles - if a PR is locked, it's locked for all "instances" of state (aka [Terraform CLI Workspaces](https://developer.hashicorp.com/terraform/cloud-docs/workspaces#terraform-cloud-vs-terraform-cli-workspaces))

state-level locks will keep working normally because are handled by terraform itself ([same as in Atlantis](https://www.runatlantis.io/docs/locking.html#relationship-to-terraform-state-locking))


## Limitations
- AWS only, for now. Not hard to add AWS / GCP support though, we just haven't yet.

## Links
- [Why are people using Terraform Cloud?](https://www.reddit.com/r/Terraform/comments/1132qf3/why_are_people_using_terraform_cloud_i_may_be/)
- [The Pains of Terraform Collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e)
- [Four Great Alternatives to HashiCorp’s Terraform Cloud](https://medium.com/@elliotgraebert/four-great-alternatives-to-hashicorps-terraform-cloud-6e0a3a0a5482)
