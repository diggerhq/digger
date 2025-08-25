---
title: CLI Reference
description: Taco command reference.
---

# CLI Reference

Global
- `--server <url>` — API endpoint (default `http://localhost:8080`).

State
- `taco state create <id>`
- `taco state ls [--prefix <pfx>]`
- `taco state rm <id>`
- `taco state pull <id> [-f <file>]`
- `taco state push <id> [-f <file>] [--if-locked-by <uuid>]`
- `taco state lock <id> [--who <str>] [--info <str>]`
- `taco state unlock <id> --id <uuid>`
- `taco state acquire <id> [-f <file>]`
- `taco state release <id> [-f <file>]`

Provider
- `taco provider init [dir] [--system-state <id>] [--no-create] [--force]`

