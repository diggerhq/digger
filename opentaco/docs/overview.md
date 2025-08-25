---
title: Overview
description: OpenTaco at a glance — what it is, what it does, and how it fits into your Terraform workflows.
---

# OpenTaco Overview

OpenTaco (Layer‑0) is a CLI + lightweight service focused on Terraform/OpenTofu state control. It provides:

- State CRUD and locking.
- An HTTP state backend proxy compatible with Terraform’s built‑in `http` backend (GET/POST/PUT/LOCK/UNLOCK).
- An SDK and Terraform provider for management operations.

Today, OpenTaco runs stateless and stores state in S3 via a “bucket‑only” adapter (with an in‑memory fallback for local demos).

Key ideas:
- Layer‑0 = state control. RBAC, policy, remote execution and automation come later.
- CLI‑first to settle semantics. UI comes later.
- Backwards‑compatible S3 layout for easy adoption.

System state convention:
- Platform‑owned IDs start with `__opentaco_`.
- Default system state: `__opentaco_system_state` (created by the CLI, not auto‑created by the service).
