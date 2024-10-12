
# Digger - CI/CD for Terraform

<img width="1470" alt="digger-opensource-gitops-banner" src="https://github.com/diggerhq/digger/assets/1280498/7fb44db3-38ca-4021-8714-87a2f1a14982">

<h2 align="center">
  <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q">Community Slack</a> |
  <a href="https://calendly.com/diggerdev/diggerdemo">Schedule a call</a> |
  <a href="https://www.loom.com/share/51f27994d95f4dc5bb6eea579e1fa8dc?sid=403f161a-6c0b-44ac-af57-cc9b56190f64">Demo Video</a> |
  <a href="https://docs.digger.dev/">Docs</a>
</h2>

## What is Digger?

**Digger** simplifies managing infrastructure using **Terraform** within your CI/CD system (like GitHub Actions). Instead of using a separate system to run Terraform (like Terraform Cloud or Atlantis), Digger runs it directly in your existing CI. This saves costs and improves security.

### Why Use Digger?

- **Secure**: Cloud access secrets stay in your system. No third-party services involved.
- **Cost-effective**: No extra charges for additional compute to run Terraform.

### Key Features

- Terraform commands (`plan`, `apply`) run directly from pull requests.
- No separate runners—your CI handles everything.
- Supports **RBAC** (role-based access control) via **Open Policy Agent (OPA)**.
- Prevents conflicts with **PR-level locks**.
- Supports **Terragrunt**, **Checkov** for static analysis, and more.
- Detects infrastructure drift.

## Getting Started

- [GitHub Actions + AWS](https://docs.digger.dev/getting-started/github-actions-+-aws)
- [GitHub Actions + GCP](https://docs.digger.dev/getting-started/github-actions-and-gcp)

## How Digger Works

Digger has two main parts:

1. **CLI Tool**: Runs inside your CI and manages Terraform commands.
2. **Orchestrator**: Makes sure jobs run smoothly and in parallel, keeping your data secure.

### Compared to Other Tools

- **Open-source**: You can host Digger yourself.
- **Unlimited usage**: No limits on the number of runs or resources you manage.
- **Runs in your CI**: No third-party servers involved, keeping secrets secure.

## Contributing

We love contributions! Check our [Contributing Guide](CONTRIBUTING.md) to get started. If you need help or have questions, join our [Slack Community](https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q).

## Telemetry

Digger collects anonymized telemetry. See [usage.go](https://github.com/diggerhq/digger/blob/develop/cli/pkg/usage/usage.go) for details. You can disable telemetry collection by setting `telemetry: false` in `digger.yml` or setting the `TELEMETRY` environment variable to `false`.

## Running Migrations

To run migrations:

```bash
atlas migrate apply --url $DATABASE_URL
```

## Resources

- [Docs](https://docs.digger.dev/) for comprehensive guides and documentation.
- [Slack](https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q) for community discussions.
- [GitHub](https://github.com/diggerhq/digger) for the source code, issues, and pull requests.
- [Medium](https://medium.com/@DiggerHQ) for articles and tutorials on Terraform automation.

