# Taco Sidecar Helm Chart

Sandbox sidecar service for OpenTaco remote Terraform/OpenTofu runs.

## Purpose

This chart deploys `sandbox-sidecar`, which is used by `taco-statesman` for E2B-backed sandbox execution.

## Integration

When enabled from the `opentaco` umbrella chart, set Statesman env values:

- `OPENTACO_SANDBOX_PROVIDER=e2b`
- `OPENTACO_E2B_SIDECAR_URL=http://<release-name>-taco-sidecar:9100`

Example with release `opentaco`:

- `http://opentaco-taco-sidecar:9100`

## External Secret Pattern

To use a pre-created secret instead of chart-managed secret values:

- set `sidecar.secret.useExistingSecret: true`
- set `sidecar.secret.existingSecretName` to your secret name

Expected secret keys:

- `SANDBOX_RUNNER`
- `E2B_API_KEY`
- `E2B_BAREBONES_TEMPLATE_ID`
