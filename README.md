<img width="1470" alt="digger-opensource-gitops-banner" src="https://github.com/diggerhq/digger/assets/1280498/7fb44db3-38ca-4021-8714-87a2f1a14982">

<h2 align="center">
  <a href="https://bit.ly/diggercommunity">Community Slack</a> |
  <a href="https://calendly.com/diggerdev/diggerdemo">Schedule a call</a> |
  <a href="https://youtu.be/vPjk3gYSxuE">Demo Video</a> |
  <a href="https://docs.digger.dev/">Docs</a>
</h2>

CI/CD for Terraform is [tricky](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). To make life easier, specialized CI systems aka [TACOS](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783) exist - Terraform Cloud, Spacelift, Atlantis, etc.

But why have 2 CI systems? Why not reuse the async jobs infrastructure with compute, orchestration, logs, etc of your existing CI?

Digger runs Terraform natively in your CI. This is:

- Secure, because cloud access secrets aren't shared with a third-party
- Cost-effective, because you are not paying for additional compute just to run your Terraform

## Features

- Terraform plan and apply in pull request comments
- Private runners - thanks to the fact that there are no separate runners! Your existing CI's compute environment is used
- Open Policy Agent (OPA) support for RBAC
- PR-level locks (on top of Terraform native state locks, similar to Atlantis) to avoid race conditions across multiple PRs
- Terragrunt, Workspaces, multiple Terraform versions, static analysis via Checkov, plan persistence, ...
- Drift detection 
  

## Getting Started

- [GitHub Actions + AWS](https://docs.digger.dev/getting-started/github-actions-+-aws)
- [GitHub Actions + GCP](https://docs.digger.dev/getting-started/github-actions-and-gcp)

## How it works

Digger has 2 main components:
- CLI that runs inside your CI and calls Terraform with the right arguments
- Orchestrator - a minimal backend (that can also be self-hosted) that triggers CI jobs in response to events such as PR comments

Digger also stores PR-level locks and plan cache in your cloud account (DynamoDB + S3 on AWS, equivalents in other cloud providers)

## Compared to Atlantis

- No need to host and maintain a server (although you [can](https://docs.digger.dev/self-host/deploy-helm))
- Secure by design: jobs run in your CI, so sensitive data stays there
- Scalable compute: jobs can run in parallel
- RBAC and policies via OPA
- Drift detection
- Apply-after-merge workflows
- Web UI (cloud-based)
- Read more about differences with Atlantis in our [blog post](https://medium.com/@DiggerHQ/digger-and-atlantis-key-differences-c08029ffe112)
​
## Compared to Terraform Cloud and other TACOs

- Open source; orchestrator can be self-hosted
- Unlimited runs and unlimited resources-under-management on all tiers
- Jobs run in your CI, not on a third-party server
- Supports PR automation (apply before merge)
- No duplication of the CI/CD stack
- Secrets not shared with a third party

## Contributing

We love contributions. Check out our [contributing guide](CONTRIBUTING.md) to get started.

Please pick an issue that already exists if you’re interested in contributing, otherwise, feel free to create an issue and triage with the maintainers before creating a PR.

Not sure where to get started? You can:

-   Join our <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q">Slack</a>, and ask us any questions there.

## Telemetry

Digger collects anonymized telemetry. See [usage.go](https://github.com/diggerhq/digger/blob/develop/cli/pkg/usage/usage.go) for detail. You can disable telemetry collection either by setting `telemetry: false` in digger.yml, or by setting the `TELEMETRY` env variable to `false`.

## Running migrations

```
atlas migrate apply --url $DATABASE_URL
```

## Resources

- [Docs](https://docs.digger.dev/) for comprehensive documentation and guides
- [Slack](https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q) for discussion with the community and Digger team.
- [GitHub](https://github.com/diggerhq/digger) for code, issues, and pull request
- [Medium](https://medium.com/@DiggerHQ) for terraform automation and collaboration insights, articles, tutorials, and updates.
