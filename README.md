<img width="1470" alt="digger-opensource-gitops-banner" src="https://github.com/diggerhq/digger/assets/1280498/7fb44db3-38ca-4021-8714-87a2f1a14982">

<h2 align="center">
  <a href="https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q">💬 Join Our Community Slack</a> |
  <a href="https://calendly.com/diggerdev/diggerdemo">📅 Schedule a Call</a> |
  <a href="https://www.loom.com/share/51f27994d95f4dc5bb6eea579e1fa8dc?sid=403f161a-6c0b-44ac-af57-cc9b56190f64">🎥 Watch Demo Video</a> |
  <a href="https://docs.digger.dev/">📚 Read Our Docs</a>
</h2>

## 🚀 Introduction

Implementing CI/CD for Terraform can be [challenging](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). Specialized CI systems (also known as [TACOS](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783)) like Terraform Cloud, Spacelift, and Atlantis exist to ease the process.

However, why manage two separate CI systems when you can reuse your existing CI for Terraform workflows?

**Digger** integrates natively with your CI, offering a secure and cost-effective solution by running Terraform within your existing CI infrastructure.

### 🔑 Key Benefits
- **🔒 Secure:** Cloud access secrets remain within your infrastructure, not shared with a third party.
- **💸 Cost-Effective:** No additional compute costs for running Terraform.

---

## ✨ Features

- 📝 Execute `terraform plan` and `terraform apply` from pull request comments.
- 🏃‍♂️ Use **private runners**—leveraging your existing CI’s compute environment.
- 🔐 Support for **Open Policy Agent (OPA)** for Role-Based Access Control (RBAC).
- 🔒 **PR-level locks** to prevent race conditions in multiple pull requests (PRs).
- 🛠️ Compatibility with **Terragrunt**, multiple **Terraform versions**, **Workspaces**, and static analysis tools like **Checkov**.
- 📈 **Drift detection** for identifying configuration discrepancies.

---

## 🛠️ Getting Started

Start using Digger with these guides:

- [GitHub Actions + AWS](https://docs.digger.dev/getting-started/github-actions-+-aws)
- [GitHub Actions + GCP](https://docs.digger.dev/getting-started/github-actions-and-gcp)

---

## 🔧 How Digger Works

Digger consists of two primary components:

1. **CLI:** This runs inside your CI, passing the correct arguments to Terraform.
2. **Orchestrator:** A minimal backend (self-hostable) that triggers CI jobs based on events (e.g., pull request comments).

Digger also uses your cloud infrastructure (e.g., DynamoDB + S3 for AWS) to store PR-level locks and the Terraform plan cache.

---

## ⚖️ Comparison with Atlantis

- **No server hosting required** (self-hosting is optional).
- **🔐 Secure by design:** Sensitive data stays within your CI environment.
- **⚡ Scalable compute:** Parallel job execution.
- **💡 RBAC via OPA**, along with **drift detection**.
- **✅ Apply-after-merge workflows** and a **Web UI** (cloud-based).

Learn more about the differences in our [blog post](https://medium.com/@DiggerHQ/digger-and-atlantis-key-differences-c08029ffe112).

---

## ⚡ Comparison with Terraform Cloud & Other TACOS

- **🆓 Open-source**, with the option to self-host the orchestrator.
- **🔄 Unlimited runs** and **unlimited resource management** on all tiers.
- **🔧 CI integration**—no need to duplicate the CI/CD stack.
- **🔒 Secrets stay within your infrastructure.**

---

## 🤝 Contributing

We welcome contributions! To get started, read our [contributing guide](CONTRIBUTING.md).

You can:

- Pick an existing issue or create a new one.
- Join our [Slack community](https://join.slack.com/t/diggertalk/shared_invite/zt-1tocl4w0x-E3RkpPiK7zQkehl8O78g8Q) and ask us any questions.

---

## 📊 Telemetry

Digger collects anonymized telemetry data. See the details in [usage.go](https://github.com/diggerhq/digger/blob/develop/cli/pkg/usage/usage.go).

To disable telemetry, set `telemetry: false` in your `digger.yml` file or use the `TELEMETRY=false` environment variable.

---

## 🛠️ Running Migrations

To run migrations, use the following command:

```bash
atlas migrate apply --url $DATABASE_URL
```
## 📚 Resources
Documentation: Comprehensive guides and references.
Slack Community: Join discussions with the Digger team and community.
GitHub: View the source code, submit issues, and contribute.
Medium: Read our insights, tutorials, and updates on Terraform automation.


