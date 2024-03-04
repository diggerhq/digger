# Digger backend Helm Chart

## Installation steps

The installation must be executed in two steps, as explaned in the [Digger official documentation](https://docs.digger.dev/self-host/deploy-docker-compose#create-github-app):

1. Install the helm chart leaving empty all the data related to the GitHub App
2. Go to `your_digger_hostname/github/setup` to install and configure the GitHub App
3. Configure in the helm values or in the external secret all the data related to the new GitHub app and upgrade the helm installation to reload the Digger application with the new configuration

## Configuration Details

To configure the Digger backend deployment with the Helm chart, you'll need to set several values in the `values.yaml` file. Below are the key configurations to consider:

- `digger.image.repository`: The Docker image repository for the Digger backend (e.g., `ghcr.io/diggerhq/digger_backend`).
- `digger.image.tag`: The specific version tag of the Docker image to deploy (e.g., `"v0.4.2"`).

- `digger.service.type`: The type of Kubernetes service to create, such as `ClusterIP`, `NodePort`, or `LoadBalancer`.
- `digger.service.port`: The port number that the service will expose (e.g., `3000`).

- `digger.ingress.enabled`: Set to `true` to create an Ingress for the service.
- `digger.annotations`: Add the needed annotations based on your ingress controller configuration.
- `digger.ingress.host`: The hostname to use for the Ingress resource (e.g., `digger-backend.test`).
- `digger.ingress.path`: The path for the Ingress resource (e.g., `/`).
- `digger.ingress.tls.secretName`: The name of the TLS secret to use for Ingress encryption (e.g., `digger-backend-tls`).

- `digger.secret.*`: Various secrets needed for the application, such as `HTTP_BASIC_AUTH_PASSWORD` and `BEARER_AUTH_TOKEN`. You can provide them directly or reference an existing Kubernetes secret by setting `useExistingSecret` to `true` and specifying `existingSecretName`.

- `digger.postgres.*`: If you're using an external Postgres database, configure the `user`, `database`, and `host` accordingly. Ensure you provide the `password` either directly or through an existing secret in the `secret.*` section.

Remember to replace placeholders and default values with your specific, sensitive information before deploying the chart. For example, it's essential to generate a strong `bearerAuthToken` and `postgresPassword` rather than using the defaults for security reasons.

You can also deploy a PostgreSQL database ONLY FOR TEST PURPOSES configuring the `postgres.*` section:
- `postgres.enabled`: Set to `true` if you want to deploy a postgres database
- `postgres.secret.*`: As for the digger secret, you can pass the `postgres` user password directly or through an existing secret
