# strate

> [!WARNING]
> This is a hard fork of Digger. This project is not affiliated with or endorsed by the original Digger team. Currently, this fork is undergoing major rewrites and may behave differently than the original. Enterprise Edition (EE) features are not present in this fork. Proper attribution to the original sources will be applied in accordance with the Apache License 2.0 requirements.

CI/CD for Terraform is [tricky](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). To make life easier, specialized CI systems aka [TACOS](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783) exist - Terraform Cloud, Spacelift, Atlantis, etc.

But why have 2 CI systems? Why not reuse the async jobs infrastructure with compute, orchestration, logs, etc of your existing CI?

Strate runs Terraform natively in your CI. This is:

- Secure, because cloud access secrets aren't shared with a third-party
- Cost-effective, because you are not paying for additional compute just to run your Terraform

## Features

- Terraform plan and apply in pull request comments
- Private runners - thanks to the fact that there are no separate runners! Your existing CI's compute environment is used
- Open Policy Agent (OPA) support for RBAC
- PR-level locks (on top of Terraform native state locks, similar to Atlantis) to avoid race conditions across multiple PRs
- Terragrunt, Workspaces, multiple Terraform versions, static analysis via Checkov, plan persistence, ...

## Getting Started

- [GitHub Actions + AWS](https://docs.digger.dev/getting-started/github-actions-+-aws)
- [GitHub Actions + GCP](https://docs.digger.dev/getting-started/github-actions-and-gcp)

## How it works

Strate has 2 main components:

- CLI that runs inside your CI and calls Terraform with the right arguments
- Orchestrator - a minimal backend (that can also be self-hosted) that triggers CI jobs in response to events such as PR comments

Strate also stores PR-level locks and plan cache in your cloud account (DynamoDB + S3 on AWS, equivalents in other cloud providers)

## Compared to Atlantis

- No need to host and maintain a server (although you [can](https://docs.digger.dev/self-host/deploy-helm))
- Secure by design: jobs run in your CI, so sensitive data stays there
- Scalable compute: jobs can run in parallel
- RBAC and policies via OPA
- Apply-after-merge workflows
- Web UI (cloud-based)
- Read more about differences with Atlantis in our [blog post](https://medium.com/@DiggerHQ/digger-and-atlantis-key-differences-c08029ffe112)

## Compared to Terraform Cloud and other TACOs

- Open source; orchestrator can be self-hosted
- Unlimited runs and unlimited resources-under-management on all tiers
- Jobs run in your CI, not on a third-party server
- Supports PR automation (apply before merge)
- No duplication of the CI/CD stack
- Secrets not shared with a third party
