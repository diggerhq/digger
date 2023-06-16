<img width="1470" alt="digger-opensource-gitops-banner" src="https://github.com/diggerhq/digger/assets/1280498/7fb44db3-38ca-4021-8714-87a2f1a14982">

<h2 align="center">
  <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q">Slack</a> |
  <a href="https://docs.digger.dev/">Docs</a>
</h2>

CI/CD for Terraform is [tricky](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). To make life easier, specialised CI systems aka [TACOS](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783) exist - Terraform Cloud, Spacelift, Atlantis, etc.

But why have 2 CI systems? Why not reuse the async jobs infrastructure with compute, orchestration, logs, etc of your existing CI?

Digger runs terraform natively in your CI and takes care of the other bits - locks, plan artifacts and so on. [Demo video](https://www.loom.com/share/e201e639a73941e0b5508710377a6106)

## Features
- Runs in any CI - Github Actions, Gitlab, Azure DevOps, etc
- Multiple VCS support - Github, Gitlab, Azure Repos, etc
- Private runners - thanks to the fact that there are no separate runners! Your existing CI's compute environment is used
- Open Policy Agent (OPA) support for RBAC
- PR-level locks (on top of Terraform native state locks, similar to Atlantis) to avoid race conditions across multiple PRs
- Terragrunt, Workspaces, multiple Terraform versions, static analysis via Checkov, plan persistence, ...
- Drift detection - coming soon
- Cost estimation - coming soon

## How to use

This is demo flow with a sample repo using local state - for real world scenario you'll need to configure remote backend (S3 + DynamoDB) and add a [workflow file](https://github.com/diggerhq/digger_demo/blob/main/.github/workflows/plan.yml) to the root of the repo.

1. Fork the [demo repository](https://github.com/diggerhq/digger_demo_multienv)
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

State-level locks will keep working normally because are handled by terraform itself ([same as in Atlantis](https://www.runatlantis.io/docs/locking.html#relationship-to-terraform-state-locking))


## Notes
- We perform anonymous usage tracking. No sensitive or personal / identifyable data is logged. You can see what is tracked in [`pkg/utils/usage.go`](https://github.com/diggerhq/digger/blob/main/pkg/utils/usage.go)

## Contributing
**If you are considering using digger within your organisation
please [reach out to us](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA).**

To contribute to Digger please follow our [Contributing guide](CONTRIBUTING.md)

## FAQ

Q) **Since you're FOSS I assume you plan to monetize by selling support? Or...?**

A) We are a vc-backed startup fully focused on this tool; in terms of monetization - we are currently in the process of launching Digger Pro. Check out the features [here](https://digger.dev/#plans) and feel free to book a [demo](https://bit.ly/diggerpro) if interested.


## Resources

- [Docs](https://docs.digger.dev/)for comprehensive documentation and guides
- [Slack](https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q) for discussion with the community and Infisical team.
- [GitHub](https://github.com/diggerhq/digger) for code, issues, and pull request
- [Medium](https://medium.com/@DiggerHQ) for terraform automation and collaboration insights, articles, tutorials, and updates.
- [Roadmap](https://diggerdev.notion.site/Digger-Roadmap-845a90fb17954afca80431580e1b3958?pvs=4) for planned features

## Links
- [The case for a 'Headless Terraform IDP' for terraform self service](https://medium.com/@DiggerHQ/the-case-for-headless-terraform-idp-5bc5a873805f)
- [Can GitHub actions be used as a CI/CD for Terraform?](https://medium.com/@DiggerHQ/can-github-actions-be-used-as-a-ci-cd-for-terraform-e4ac59a38b0)
- [Why are people using Terraform Cloud?](https://www.reddit.com/r/Terraform/comments/1132qf3/why_are_people_using_terraform_cloud_i_may_be/)
- [The Pains of Terraform Collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e)
- [Four Great Alternatives to HashiCorp’s Terraform Cloud](https://medium.com/@elliotgraebert/four-great-alternatives-to-hashicorps-terraform-cloud-6e0a3a0a5482)
