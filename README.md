<img width="1470" alt="digger-opensource-gitops-banner" src="https://github.com/diggerhq/digger/assets/1280498/7fb44db3-38ca-4021-8714-87a2f1a14982">

<h2 align="center">
  <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q">Slack</a> |
  <a href="https://docs.digger.dev/">Docs</a>
  <a href="https://www.loom.com/share/51f27994d95f4dc5bb6eea579e1fa8dc?sid=403f161a-6c0b-44ac-af57-cc9b56190f64">Demo Video</a>
</h2>

CI/CD for Terraform is [tricky](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). To make life easier, specialised CI systems aka [TACOS](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783) exist - Terraform Cloud, Spacelift, Atlantis, etc.

But why have 2 CI systems? Why not reuse the async jobs infrastructure with compute, orchestration, logs, etc of your existing CI?

Digger runs terraform natively in your CI and takes care of the other bits - locks, plan artifacts and so on.

## Features
- See terraform plan and run apply as pull request comments
- Any VCS - Github, Gitlab, Azure Repos, etc
- Any CI - Github Actions, Gitlab, Azure DevOps, etc
- Any cloud provider - AWS, GCP, Azure
- Private runners - thanks to the fact that there are no separate runners! Your existing CI's compute environment is used
- Open Policy Agent (OPA) support for RBAC
- PR-level locks (on top of Terraform native state locks, similar to Atlantis) to avoid race conditions across multiple PRs
- Terragrunt, Workspaces, multiple Terraform versions, static analysis via Checkov, plan persistence, ...
- Drift detection - coming soon
- Cost estimation - coming soon

## Getting Started

- [Github Actions + AWS](https://docs.digger.dev/getting-started/github-actions-+-aws)
- [Github Actions + GCP](https://docs.digger.dev/getting-started/github-actions-and-gcp)
- [Gitlab Pipelines + GCP](https://docs.digger.dev/getting-started/gitlab-pipelines-+-aws)
- [Azure DevOps](https://docs.digger.dev/getting-started/azure-devops)

## How it works

Digger has 2 main components:
- CLI that runs inside your CI and calls terraform with the right arguments
- Orchestrator - a minimal backend (that can also be self-hosted) that triggers CI jobs in response to events such as PR comments

Digger also stores PR-level locks and plan cache in your cloud account (DynamoDB + S3 on AWS, equivalents in other cloud providers)

## Telementry
No sensitive or personal / identifyable data is logged. You can see what is tracked in [`pkg/utils/usage.go`](https://github.com/diggerhq/digger/blob/main/pkg/utils/usage.go)

## Contributing
**If you are considering using digger within your organisation
please [reach out to us](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA).**

To contribute to Digger please follow our [Contributing guide](CONTRIBUTING.md)

## Resources

- [Docs](https://docs.digger.dev/) for comprehensive documentation and guides
- [Slack](https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q) for discussion with the community and Infisical team.
- [GitHub](https://github.com/diggerhq/digger) for code, issues, and pull request
- [Medium](https://medium.com/@DiggerHQ) for terraform automation and collaboration insights, articles, tutorials, and updates.
- [Roadmap](https://diggerdev.notion.site/Digger-Roadmap-845a90fb17954afca80431580e1b3958?pvs=4) for planned features
