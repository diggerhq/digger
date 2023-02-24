# tfrun
A Github Action that runs Terraform `plan` and `apply` with PR-level locks

Just like Atlantis - but without a self-hosted backend, and terraform runs in GH actions

## Features
- code-level locks - only 1 open PR can run plan / apply. This avoids conflicts
- no need to install any backend into your infra - locks are stored in DynamoDB

## How to use

1. clone the demo terraform repository (or use your own repo with terraform)
2. (configure S3 backend + dynamo) (script?) (should get table)
3. Add environment variables into your Github Action Secrets
- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY
- DYNAMODB_LOCKS_TABLE (from step 2)
4. if you are using your own repo: add a [workflow file](https://github.com/diggerhq/tfrun_demo/blob/main/.github/workflows/plan.yml) to the root of the repo
5. make a change and create a PR - this will create a lock
6. comment `digger plan` - terraform plan output will be added as comment
7. create another PR - plan or apply won’t work in this PR until the first lock is released
8. you should see `Locked by PR #1` comment

## Limitations
- AWS only, for now. Not hard to add AWS / GCP support though, we just haven't yet.
- Only `plan`, no `apply` yet (coming soon) - we wanted to validate the locks mechanism first. 

## Links
- [Why are people using Terraform Cloud?](https://www.reddit.com/r/Terraform/comments/1132qf3/why_are_people_using_terraform_cloud_i_may_be/)
- [The Pains of Terraform Collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e)
- [Four Great Alternatives to HashiCorp’s Terraform Cloud](https://medium.com/@elliotgraebert/four-great-alternatives-to-hashicorps-terraform-cloud-6e0a3a0a5482)
