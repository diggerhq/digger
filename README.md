# tfrun
A Github Action that runs Terraform `plan` and `apply` with PR-level locks

<img width="693" alt="Screenshot 2023-02-24 at 19 52 12" src="https://user-images.githubusercontent.com/1280498/221277610-368ae950-6319-4bf3-9df2-ca75ca5a05f9.png">

Just like Atlantis - but without a self-hosted backend, and terraform binary runs in GH actions compute environment

## Features
- code-level locks - only 1 open PR can run plan / apply. This avoids conflicts
- no need to install any backend into your infra - locks are stored in DynamoDB

## How to use

This is demo flow with a sample repo using local state - for real world scenario you'll need to configure remote backend (S3 + DynamoDB) and add a [workflow file](https://github.com/diggerhq/tfrun_demo/blob/main/.github/workflows/plan.yml) to the root of the repo.

1. clone the [demo repository](https://github.com/diggerhq/tfrun_demo) and push to your Github

DO NOT FORK - Github Actions auto-comments will not work in a fork, even if you enable actions manually

2. Add environment variables into your Github Action Secrets
- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY
3. make a change and create a PR - this will create a lock
4. comment `digger plan` - terraform plan output will be added as comment
5. create another PR - plan or apply won’t work in this PR until the first lock is released
6. you should see `Locked by PR #1` comment

## Remote backend and state-level locks

tfrun does not interfere with your remote backend setup. You could be using [S3 backend](https://developer.hashicorp.com/terraform/language/settings/backends/s3) or TF cloud's [remote backend](https://developer.hashicorp.com/terraform/language/settings/backends/remote) or [some other way](https://developer.hashicorp.com/terraform/language/settings/backends/configuration)

tfrun also doesn't differentiate locks based on statefiles - if a PR is locked, it's locked for all "instances" of state (aka [Terraform CLI Workspaces](https://developer.hashicorp.com/terraform/cloud-docs/workspaces#terraform-cloud-vs-terraform-cli-workspaces))

state-level locks will keep working normally because are handled by terraform itself ([same as in Atlantis](https://www.runatlantis.io/docs/locking.html#relationship-to-terraform-state-locking))


## Limitations
- AWS only, for now. Not hard to add AWS / GCP support though, we just haven't yet.
- Only `plan`, no `apply` yet (coming soon) - we wanted to validate the locks mechanism first. 

## Links
- [Why are people using Terraform Cloud?](https://www.reddit.com/r/Terraform/comments/1132qf3/why_are_people_using_terraform_cloud_i_may_be/)
- [The Pains of Terraform Collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e)
- [Four Great Alternatives to HashiCorp’s Terraform Cloud](https://medium.com/@elliotgraebert/four-great-alternatives-to-hashicorps-terraform-cloud-6e0a3a0a5482)
