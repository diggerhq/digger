# Digger

<h1 align="center">
  <img width="733" alt="Screenshot 2023-02-28 at 11 25 48" src="https://user-images.githubusercontent.com/1280498/221849642-ae6cb056-5b5b-478f-8cfb-42790e1739e7.png">
</h1>
<p align="center">
  <p align="center">Digger is an open-source Terraform Cloud Alternative</p>
</p>
<h2 align="center">
  <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA">Slack</a> |
  <a href="https://digger.dev">Website</a>
</h2>

Digger is Github Action that runs Terraform `plan` and `apply` with PR-level locks

Unlike Terraform Cloud or Spacelift, terraform jobs run natively in your Github Actions - no need to share sensitive data with another CI system

Unlike Atlantis, there's no need to deploy and maintain a backend service.

<img width="693" alt="Screenshot 2023-02-24 at 19 52 12" src="https://user-images.githubusercontent.com/1280498/221277610-368ae950-6319-4bf3-9df2-ca75ca5a05f9.png">

Demo video: https://www.loom.com/share/e201e639a73941e0b5508710377a6106

## Features
- code-level locks - only 1 open PR can run plan / apply. This avoids conflicts
- no need to install any backend into your infra - locks are stored in DynamoDB

## How to use

This is demo flow with a sample repo using local state - for real world scenario you'll need to configure remote backend (S3 + DynamoDB) and add a [workflow file](https://github.com/diggerhq/digger_demo/blob/main/.github/workflows/plan.yml) to the root of the repo.

1. Fork the [demo repository](https://github.com/diggerhq/digger_demo)
2. Enable Actions (by default workflows won't trigger in a fork)

<img width="1441" alt="Screenshot 2023-02-24 at 20 24 08" src="https://user-images.githubusercontent.com/1280498/221291130-6831d45a-008f-452f-91d3-37ba133d7cbb.png">


2. In your repository settings > Actions ensure that the Workflow Read and Write permissions are assigned - This will allow the workflow  to post comments on your PRs
<img width="1017" alt="Screen Shot 2023-03-01 at 12 02 59 PM" src="https://user-images.githubusercontent.com/1627972/222136385-c7cb8f2c-1731-475d-b3a4-78b0d79a3874.png">

3. Add environment variables into your Github Action Secrets (NOTE: This step is optional if you just want to test out the Action with `null_resource`)
- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY 
4. make a change and create a PR - this will create a lock
5. comment `digger plan` - terraform plan output will be added as comment. If you don't see a comment (bug) - check out job output
6. create another PR - plan or apply won’t work in this PR until the first lock is released
7. you should see `Locked by PR #1` comment. The action logs will display "Project locked" error message.

## Remote backend and state-level locks

Digger does not interfere with your remote backend setup. You could be using [S3 backend](https://developer.hashicorp.com/terraform/language/settings/backends/s3) or TF cloud's [remote backend](https://developer.hashicorp.com/terraform/language/settings/backends/remote) or [some other way](https://developer.hashicorp.com/terraform/language/settings/backends/configuration)

Digger also doesn't differentiate locks based on statefiles - if a PR is locked, it's locked for all "instances" of state (aka [Terraform CLI Workspaces](https://developer.hashicorp.com/terraform/cloud-docs/workspaces#terraform-cloud-vs-terraform-cli-workspaces))

state-level locks will keep working normally because are handled by terraform itself ([same as in Atlantis](https://www.runatlantis.io/docs/locking.html#relationship-to-terraform-state-locking))


## Roadmap

- Support for multiple modes of locking (apply-only, no-lock + queing)
- 🔍 GCP Support
    - Supporting of GCP storage buckets for PR locks
- 🔍 Azure Support 
    - Supporting of Azure Cosmos DB for PR Locks
- 🔍 Gitlab Support
- 🔍 Jenkins Support


## Notes
- We perform anonymous usage tracking. No sensitive or personal / identifyable data is logged. You can see what is tracked in [`pkg/utils/usage.go`](https://github.com/diggerhq/digger/blob/main/pkg/utils/usage.go)

## Contributing
**If you are considering using digger within your organisation
please [reach out to us](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA).**

To contribute to Digger please follow our [Contributing guide](CONTRIBUTING.md)

## Links
- [Why are people using Terraform Cloud?](https://www.reddit.com/r/Terraform/comments/1132qf3/why_are_people_using_terraform_cloud_i_may_be/)
- [The Pains of Terraform Collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e)
- [Four Great Alternatives to HashiCorp’s Terraform Cloud](https://medium.com/@elliotgraebert/four-great-alternatives-to-hashicorps-terraform-cloud-6e0a3a0a5482)
