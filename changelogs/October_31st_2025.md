# 10/31

We’re excited to be launching next week! Here are the highlights of what went down in the last few days at Digger!

Latest Version: **v0.6.131**

# **PR #2326: Helm charts for OpenTaco and UI deployment adjustments**

> by @breardon2011

In our continued pursuit of being “Self hosted first”, this PR Introduced complete Helm chart support for deploying OpenTaco, including modular charts for backend, UI, drift detection, and states management services. 

The PR also Added CI workflows for automated Docker image builds and GitHub releases for backend, UI, and drift components, along with updating the OpenTaco umbrella chart with test and production-ready values, integrated Cloud SQL and secret management examples, and improved documentation for setup and deployment. 

Adjusted the UI deployment process to use container-based builds instead of Netlify and optimized .dockerignore to exclude unnecessary files. Overall, this release enables one-command Kubernetes deployment for OpenTaco with clearer configuration and packaging automation. Something we’ve been striving to achieve for a while - BIG WIN!

# **PR #2343: tfe units ui functionalities**

> by @motatoes

This update introduces full TFE compatible endpoints and UI support across the OpenTaco stack. On the backend, it adds duplicate-user validation via a new GetUserByEmail DB helper, improves internal user creation, and enhances TFE request handling by honouring forwarded headers behind reverse proxies. The unit API now skips unstable dependency graph updates. In the frontend, major enhancements include expanded Statesman API integration (unit locking/unlocking, version restore, state upload/download, deletion), new /tfe/* proxy routes and .well-known/terraform.json endpoint for Terraform client compatibility, and a redesigned dashboard with nested settings pages for “User” and “API Tokens.” Additionally, WorkOS auth flows now sync organizations and users automatically to backend and Statesman services, and a new “Force Push State” dialog enables manual state uploads. 

Together, these changes make OpenTaco’s UI and APIs production-ready for Terraform Cloud-compatible automation and state management. 

L.F.G

# **PR #2349: Update Helm Chart Release Workflow**

> by @breardon2011

This update restructures the GitHub Actions workflow for releasing Helm charts. It splits the process into two stages:

1. **`release-charts`** job — packages and pushes individual component charts (`digger-backend`, `taco-orchestrator`, `taco-statesman`, `taco-token-service`, `taco-drift`, `taco-ui`) to the GitHub Container Registry (`ghcr.io/diggerhq/helm-charts`).
2. **`release-umbrella`** job**:**  runs after all components are published, rebuilding dependencies for the `opentaco` umbrella chart and pushing it as a final aggregate release.

This ensures that the main OpenTaco Helm chart always references up-to-date component dependencies and improves reliability of automated multi-chart publishing.

We’re HEADS DOWN focused on the launch! See you next week!
