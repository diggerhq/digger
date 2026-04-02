# Docker Compose self-hosting

This directory contains the top-level local self-hosting Docker Compose stack.

## Quick start

```bash
# from self-hosting/docker-compose/
make up PROFILE=platform
make up PROFILE=opentaco
make up PROFILE=all

# convenience aliases
make platform-up
make opentaco-up
make all-up
```

`PROFILE=opentaco` now starts only the OpenTACO app services. This is useful when databases/object storage are external to this compose stack.

If you want local infra too, either:

```bash
make up PROFILE=all
```

or start platform and app profiles separately:

```bash
make up PROFILE=platform
make up PROFILE=opentaco
```

## Direct usage

```bash
make -C self-hosting/docker-compose up PROFILE=platform PROJECT_ROOT=$(pwd)
```

## Rebuild flows

```bash
make build PROFILE=all
make up-build PROFILE=all
make recreate PROFILE=all
make rebuild PROFILE=all
```

The Makefile sets `--project-directory` to the repository root. In that mode, Compose resolves paths from the repo root, so `env_file` entries use `self-hosting/docker-compose/*.env`, and build contexts use repo-root paths like `.`, `taco`, and `sandbox-sidecar`.

## Env file examples

Service-specific env examples live in this directory:

- `orchestrator.env.example`
- `drift.env.example`
- `sidecar.env.example`
- `ui.env.example`

Copy them to the `env_file` targets used by compose:

```bash
cp self-hosting/docker-compose/orchestrator.env.example self-hosting/docker-compose/orchestrator.env
cp self-hosting/docker-compose/drift.env.example self-hosting/docker-compose/drift.env
cp self-hosting/docker-compose/sidecar.env.example self-hosting/docker-compose/sidecar.env
cp self-hosting/docker-compose/ui.env.example self-hosting/docker-compose/ui.env
```

Each service now reads its own env file directly from `self-hosting/docker-compose/`.

## Drift scheduler

The compose stack includes a `drift-scheduler` service that periodically calls the drift internal endpoints:

- `/_internal/process_drift`
- `/_internal/process_notifications`

Keep `drift-scheduler` `DIGGER_WEBHOOK_SECRET` aligned with the `drift` service `DIGGER_WEBHOOK_SECRET` value.
