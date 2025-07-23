<img width="1470" alt="digger-opensource-gitops-banner" src="https://github.com/diggerhq/digger/assets/1280498/7fb44db3-38ca-4021-8714-87a2f1a14982">

CI/CD for Terraform is [tricky](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). To make life easier, specialised CI systems aka [TACOS](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783) exist - Terraform Cloud, Spacelift, Atlantis, etc.

But why have 2 CI systems? Why not reuse the async jobs infrastructure with compute, orchestration, logs, etc of your existing CI?

Digger runs terraform natively in your CI. This is:

- Secure, because cloud access secrets aren't shared with a third-party
- Cost-effective, because you are not paying for additional compute just to run your terraform

## Differences Compared to Atlantis

- No need to host and maintain a server
- Secure by design
- Scalable compute with jobs isolation
- Role-based access control via OPA
- Read more about differences with Atlantis in our blog post
​
## Compared to Terraform Cloud and other TACOs

- Open source
- No duplication of the CI/CD stack
- Secrets not shared with a third party
​
## Support for other CI’s

We are currently designing Digger to be Multi-CI, so that in addition to GitHub Actions, you can run Terraform/OpenTofu within other CI’s such as Gitlab CI, Azure DevOps, Bitbucket, TeamCity, Circle CI and Jenkins, while still having the option to orchestrate jobs using Digger’s Orchestrator Backend.

Read more in this [blog](https://blog.digger.dev/how-we-are-designing-digger-to-support-multiple-ci-systems/), and please share your requirement on [Slack](https://bit.ly/diggercommunity) if you require support for other CI’s. Your feedback/insight would help us a lot as this feature is in active development.



# Digger Backend Helm Chart

## Installation steps

The installation must be executed in two steps, as explaned in the [Digger official documentation](https://docs.digger.dev/self-host/deploy-docker-compose#create-github-app):

1. Install the `digger-backend` helm chart from https://diggerhq.github.io/helm-charts/, leaving empty all the data related to the GitHub App
2. Go to `your_digger_hostname/github/setup` to install and configure the GitHub App
3. Configure in the helm values or in the external secret all the data related to the new GitHub app and upgrade the helm installation to reload the Digger application with the new configuration

## Configuration Details

To configure the Digger backend deployment with the Helm chart, you'll need to set several values in the `values.yaml` file. Below are the key configurations to consider:

- `digger.image.repository`: The Docker image repository for the Digger backend (e.g., `registry.digger.dev/diggerhq/digger_backend`).
- `digger.image.tag`: The specific version tag of the Docker image to deploy (e.g., `"v0.4.2"`).

- `digger.service.type`: The type of Kubernetes service to create, such as `ClusterIP`, `NodePort`, or `LoadBalancer`.
- `digger.service.port`: The port number that the service will expose (e.g., `3000`).

- `digger.ingress.enabled`: Set to `true` to create an Ingress for the service.
- `digger.annotations`: Add the needed annotations based on your ingress controller configuration.
- `digger.ingress.host`: The hostname to use for the Ingress resource (e.g., `digger-backend.test`).
- `digger.ingress.path`: The path for the Ingress resource (e.g., `/`).
- `digger.ingress.className`: the classname to use for ingress (only considered for kuberetes >= 1.18)
- `digger.ingress.tls.secretName`: The name of the TLS secret to use for Ingress encryption (e.g., `digger-backend-tls`).

- `digger.secret.*`: Various secrets needed for the application, such as `HTTP_BASIC_AUTH_PASSWORD` and `BEARER_AUTH_TOKEN`. You can provide them directly or reference an existing Kubernetes secret by setting `useExistingSecret` to `true` and specifying `existingSecretName`.

- `digger.postgres.*`: If you're using an external Postgres database, configure the `user`, `database`, and `host` accordingly. Ensure you provide the `password` either directly or through an existing secret in the `secret.*` section.

Remember to replace placeholders and default values with your specific, sensitive information before deploying the chart. For example, it's essential to generate a strong `bearerAuthToken` and `postgresPassword` rather than using the defaults for security reasons.

You can also deploy a PostgreSQL database ONLY FOR TEST PURPOSES configuring the `postgres.*` section:

- `postgres.enabled`: Set to `true` if you want to deploy a postgres database
- `postgres.secret.*`: As for the digger secret, you can pass the `postgres` user password directly or through an existing secret
