<h1 align="center">
  <img width="733" alt="Screenshot 2023-02-28 at 11 25 48" src="https://user-images.githubusercontent.com/1280498/221849642-ae6cb056-5b5b-478f-8cfb-42790e1739e7.png">
</h1>
<h2 align="center">
  <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q">Slack</a> |
  <a href="https://digger.dev">Website</a> |
  <a href="https://docs.digger.dev/">Docs</a>
</h2>

Digger runs Terraform jobs in the CI/CD system you already have, such as Github Actions.

CI/CD for Terraform is [not easy](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). This is why Terraform Cloud, Spacelift and Atlantis are essentially standalone full-stack CI/CD systems.

But why have 2 CI systems? Why not reuse the existing CI infrastructure? Digger does just that.

With Digger terraform jobs natively in your CI runners. It takes care of locks, state, outputs etc. [Demo video](https://www.loom.com/share/e201e639a73941e0b5508710377a6106)


## Features
- 👟 Runner-less. Terraform runs in the compute environment of your existing CI such as Github Actions, Gitlab, Argo etc.
- 🪶 Minimal / no backend. Digger's own backend is a serverless function; it is only needed for certain CI environments (eg Gitlab)
- 🔒 Code-level locks. Avoid race conditions across multiple PRs. Similar to Atlantis workflow.
- ☁️ Multi-cloud. At the moment Digger supports AWS and GCP; Azure support coming in April 2023 (yes, in a few weeks).
- 💥 Projects. Allow to isolate terraform runs and locks to a specific directory
- 💥 Terragrunt support
- 💥 Workspaces support

## Roadmap

Need a feature that's not listed? Book a [community feedback call](https://calendly.com/diggerdev/digger-community-feedback) - we ship fast ✅

- ✅ GCP support. Store PR locks in GCP storage buckets. Shipped in [#50](https://github.com/diggerhq/digger/pull/50)
- ✅ Workspaces support. Allow usage of Terraform CLI Workspaces. Shipped in [#72](https://github.com/diggerhq/digger/pull/72)
- ✅ Terragrunt support. Config option to run terragrunt wrapper. Shipped in [#76](https://github.com/diggerhq/digger/pull/76)
- ✅ Azure support using Storage Account Tables WIP: [#122](https://github.com/diggerhq/digger/pull/122)
- ⌚ AWS CodeBuild support
- ⌛ Gitlab Support
- ⌛ Configurable workflows. In addition to Atlantis-style (apply, then merge) also support "apply-only" and "no-lock"
- ⌛ Bitbucket Support
- ⌛ Jenkins Support

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


## Notes
- We perform anonymous usage tracking. No sensitive or personal / identifyable data is logged. You can see what is tracked in [`pkg/utils/usage.go`](https://github.com/diggerhq/digger/blob/main/pkg/utils/usage.go)

## Contributing
**If you are considering using digger within your organisation
please [reach out to us](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA).**

To contribute to Digger please follow our [Contributing guide](CONTRIBUTING.md)

## FAQ

Q) **Since you're FOSS I assume you plan to monetize by selling support? Or...?**

A) We are a vc-backed startup fully focused on this tool; in terms of monetization not planning to reinvent the wheel - we're just going to introduce an "enterprise tier" later on with things like OPA integration, drift detection, cost control, multi-team dashboards etc etc. And yes - support. Similarly to what Signoz does for monitoring, or Posthog for product metrics.

## Links
- [Why are people using Terraform Cloud?](https://www.reddit.com/r/Terraform/comments/1132qf3/why_are_people_using_terraform_cloud_i_may_be/)
- [The Pains of Terraform Collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e)
- [Four Great Alternatives to HashiCorp’s Terraform Cloud](https://medium.com/@elliotgraebert/four-great-alternatives-to-hashicorps-terraform-cloud-6e0a3a0a5482)
