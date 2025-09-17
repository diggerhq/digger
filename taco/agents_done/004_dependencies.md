Here’s a single, copy-paste task spec you can hand to your agent. It bakes in your updates:

* The **graph workspace** is named **`__opentaco_system_state`** (already exists).
* **Only hashes** are stored for dependency edges (no plaintext values) to keep it simple.
* The **only stateful store** for the dependency graph is the **normal Terraform state** of that workspace.

---

# OpenTaco — Output-Level Dependencies via `opentaco_dependency`

**Goal:** implement explicit, output-level dependencies between Terraform states so we can compute **state status** (`red` / `yellow` / `green`) and show it in `taco state status`, without adding any new database. All metadata lives in the **Terraform state** of a dedicated workspace named **`__opentaco_system_state`**.

**Key idea:** one Terraform resource **per edge** (`opentaco_dependency`). Each edge stores **digests only**:

* `in_digest`: the latest hash of the source output,
* `out_digest`: the last acknowledged hash at the target,
* `status`: `"pending" | "ok" | "unknown"`,
* timestamps for when each side changed/ack’d.

Service logic updates these on **state writes** (source refreshes `in*`, target acknowledges by copying `out* ← in*`).

---

## Definitions

* **State ID**: the S3 key path of a state, e.g. `org/app/prod/terraform.tfstate`.
* **Source (from)**: `(from_state_id, from_output)` — the state/output providing a value.
* **Target (to)**: `(to_state_id, to_input)` — the consumer state (input name is for docs/UX).
* **Edge**: a dependency from a *source output* to a *target input*, represented by one `opentaco_dependency` resource.
* **Digest**: `sha256(base64url)` of the **canonical JSON** representation of the output value (see hashing below).

---

## Where the graph lives

* Graph resources live in a **single Terraform workspace/project** using your normal OpenTaco backend:

  * Workspace name (project path): **`__opentaco_system_state`**
  * Its state file (graph state) is just another object in your S3 bucket and is locked/updated via your S3-compatible backend (same auth/RBAC).

> This is the **only** place we persist dependency data.

---

## Terraform Provider — `opentaco_dependency` resource

Implement a new resource in the provider. **One edge per output**.

```hcl
# Example
resource "opentaco_dependency" "a_to_b_dburl" {
  from_state_id = "org/app/A/terraform.tfstate"
  from_output   = "db_url"

  to_state_id   = "org/app/B/terraform.tfstate"
  to_input      = "db_url"   # optional, for documentation/UX

  # Computed (do not set in HCL; stored as hashes only)
  #   in_digest:  digest of latest source output value
  #   out_digest: digest last acknowledged by target
  #   status:     "pending" (in≠out), "ok" (in=out), "unknown" (source output missing)
  #   last_in_at / last_out_at: RFC3339 UTC timestamps
}
```

**Resource identity (ID):** deterministic:

```
edge_id = base64url( sha256(
  from_state_id + "\n" + from_output + "\n" + to_state_id + "\n" + to_input
))
```

Provider sets `id = edge_id`. The service uses this ID to find/update the instance inside the graph state.

**Schema (provider):**

* Required: `from_state_id (string)`, `from_output (string)`, `to_state_id (string)`
* Optional: `to_input (string)`
* Computed: `in_digest (string)`, `out_digest (string)`, `status (string)`, `last_in_at (string RFC3339)`, `last_out_at (string RFC3339)`

**Provider behavior (v1):**

* **Create/Update/Read/Delete** affect only the graph state (no remote calls needed).
* **No plaintext values** are ever stored; provider never prints values.
* On create, leave `out_digest` empty → `status="pending"` until target applies (see service flow).
  *(If you later want “start green,” add a flag to seed `out_digest=in_digest`; not in v1.)*

---

## Hashing (canonical JSON → digest)

* Canonicalize any Terraform output value to **canonical JSON**:

  * Sort object keys lexicographically, no insignificant whitespace.
  * Numbers as JSON numbers, strings UTF-8, arrays in order.
  * (Simple RFC-8785-like approach is fine; we just need stable bytes.)
* **Digest**: `SHA-256` of canonical JSON bytes; store as **base64url** string (e.g., `sha256-<base64url>` or just `<base64url>` — pick one format and stick with it).
* Equality check is **digest equality only**.

---

## Service responsibilities (state write hooks)

Whenever **any** state `S` is written via the S3-compatible backend (you already have the interception point), perform:

### A) Update **outgoing** edges (source refresh)

* Find all edges with `from_state_id == S`.
* For each edge:

  * If `from_output` exists in `S.outputs`:

    * Compute `in_digest` from that output value.
    * Set `last_in_at = now()`.
    * Recompute `status = (in_digest == out_digest ? "ok" : "pending")`.
  * Else:

    * Set `status = "unknown"` (leave digests unchanged).
* Persist changes by **editing the graph tfstate** (see “State surgery”).

### B) Update **incoming** edges (target acknowledge)

* Find all edges with `to_state_id == S`.
* For each edge:

  * If `in_digest` is set (non-empty):

    * Set `out_digest = in_digest`.
    * Set `last_out_at = now()`.
    * Set `status = "ok"`.
  * Else:

    * Leave as is (likely still `"unknown"`).
* Persist via the same **graph tfstate** edit.

**Atomicity & concurrency**

* Acquire a **Terraform state lock** for `__opentaco_system_state` using your backend’s lock (you already support S3-lock).
* Read → modify **in memory** → bump the state `serial` → write → unlock.
* Batch all edge updates for this state write in a **single** read/modify/write to avoid thrashing.

**Finding edges efficiently**

* v1: **scan the graph state** (`resources[].type == "opentaco_dependency"`) and inspect each `instances[].attributes`. For hundreds/thousands of edges this is fine.
* (Optional later) maintain a **non-authoritative** S3 index (`.taco/graph/by-from/<state_id>`, `.taco/graph/by-to/<state_id>`) to pre-filter candidate IDs; still treat the tfstate as **source of truth**.

---

## State surgery (Terraform 1.x state JSON)

* Locate resource blocks with `type="opentaco_dependency"`.
* For each `instances[]`, match by **`attributes.from_state_id`** or **`attributes.to_state_id`**.
* Update `attributes.in_digest`, `attributes.out_digest`, `attributes.status`, `attributes.last_in_at`, `attributes.last_out_at`.
* Increment top-level `serial`; preserve `lineage` and other fields.
* Write a new object/version via the backend (your service already knows how to write a state file safely).

---

## Status semantics

### Edge (`opentaco_dependency.status`)

* `"ok"`       → `in_digest == out_digest` and non-empty
* `"pending"`  → both are set and unequal
* `"unknown"`  → source output missing or `in_digest` unset (can’t decide)

### State (derived at query time)

* **red**    → the state has **any incoming edge** with `status="pending"`.
* **yellow** → the state has **no incoming pending**, but there exists **any upstream state** (transitively via edges) that is **red**.
* **green**  → neither red nor yellow.

> **Yellow propagation:** build adjacency from edges (`from_state_id → to_state_id`) and do a BFS/DFS from **red** states to mark downstream as yellow (without overwriting red).

---

## API additions

* `GET /v1/states/:id/status`

  * Computes and returns state status and its **incoming** edge statuses:

  ```json
  {
    "state_id": "org/app/B/terraform.tfstate",
    "status": "red|yellow|green",
    "incoming": [
      {
        "edge_id": "…",
        "from_state_id": "org/app/A/terraform.tfstate",
        "from_output": "db_url",
        "status": "pending|ok|unknown",
        "in_digest": "…",
        "out_digest": "…",
        "last_in_at": "…",
        "last_out_at": "…"
      }
    ],
    "summary": { "incoming_ok": N, "incoming_pending": M, "incoming_unknown": K }
  }
  ```
* (Optional) `GET /v1/graph/status`

  * Returns status of all states with red/yellow propagation for a global view.

*(Both endpoints read the **graph tfstate**; no other storage.)*

---

## CLI additions

* `taco state status [<state-id> | --prefix org/app/]`

  * Shows a table: **State | Status | Pending edges | First offender**.
  * `-o json` returns the API payload.
* (Optional) `taco dep ls [--from <state> | --to <state>]` to browse edges.

---

## Acceptance criteria

* **Provider**

  * `opentaco_dependency` resource available; `id` is deterministic; computed fields present.
  * Creating an edge does **not** store plaintext values anywhere; only digests and timestamps.

* **Service**

  * On writing state `A`: updates **outgoing** edges’ `in_digest/status`.
  * On writing state `B`: updates **incoming** edges’ `out_digest/status` to acknowledge.
  * Graph tfstate locking respected; one batched write per state update event.
  * `GET /v1/states/:id/status` reports:

    * `red` when any incoming edge pending,
    * `yellow` when upstream red exists,
    * `green` otherwise.

* **CLI**

  * `taco state status` shows correct colors for sample chains (A→B→C).
  * `-o json` returns machine-readable structure.

* **Security**

  * **No plaintext output values** are stored or logged.
  * Only digests and timestamps are persisted in the graph tfstate.
  * RBAC applies to both the graph state surgery and status reads (admin or appropriately scoped roles).

* **Perf**

  * A change in state with N dependent edges yields **one** lock + write, and completes within reasonable time for hundreds of edges.

---

## Test plan (minimum)

1. **Single edge**

   * A→B on `db_url`.
   * A apply → edge `pending`; B status `red`.
   * B apply → edge `ok`; B status `green`.

2. **Missing output**

   * Edge references `from_output` not present in A → status `unknown`; B status `green` (unless other edges are pending).

3. **Chain propagation**

   * A→B, B→C.
   * A pending → B `red`, C `yellow`.
   * B apply → A pending remains, B `green` (if its incoming now ok), C still `yellow`.
   * A apply then B apply → all green.

4. **Concurrency**

   * Rapid consecutive writes to A; ensure last digest wins, graph tfstate remains consistent (serial increments, no torn writes).

5. **Large edge fan-out**

   * A feeds 200 edges; one A apply updates all in a single batched graph write.

---

## Implementation notes & guardrails

* **Locking:** use your S3-compatible backend’s lock for `__opentaco_system_state`. If the graph workspace is also managed via Terraform elsewhere, the lock prevents clashes.
* **Error handling:** if graph tfstate is missing or corrupt, log and continue the state write (don’t fail user apply), and surface a warning in status endpoints.
* **ID changes:** if an edge’s identity inputs change (from\*/to\*), Terraform will destroy/create; the service just reacts to the new resource.
* **RBAC:** treat graph tfstate operations as **admin**; status reads require `state.read` on the target state (and perhaps on the source for including digests—digests are not sensitive, but follow least privilege).
* **Extensibility:** later you can add `plaintext` flags or richer metadata without changing the flow.

---

## Deliverables (agent checklist)

* [ ] Provider: `opentaco_dependency` resource with schema above; deterministic `id`.
* [ ] Service: hook on state write → update graph tfstate for outgoing/incoming edges (batched, locked).
* [ ] Service: `GET /v1/states/:id/status` (and optional `/v1/graph/status`).
* [ ] CLI: `taco state status` with colors and `-o json`.
* [ ] Tests: single edge, missing output, chain propagation, concurrency/fan-out.
* [ ] Docs: brief README section showing how to declare edges (in `__opentaco_system_state`) and how `red`/`yellow`/`green` are computed.

---

## Example HCL (for the graph workspace)

```hcl
# __opentaco_system_state/main.tf

terraform {
  backend "s3" {
    bucket  = "opentaco"
    key     = "__opentaco_system_state/terraform.tfstate"
    region  = "auto"
    endpoints = { s3 = "https://s3.opentaco.example" }
    use_path_style              = true
    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    use_lockfile                = true
    profile = "opentaco-state-backend"
  }
}

# Example edges (one per output)
resource "opentaco_dependency" "a_dburl_to_b" {
  from_state_id = "org/app/A/terraform.tfstate"
  from_output   = "db_url"

  to_state_id   = "org/app/B/terraform.tfstate"
  to_input      = "db_url"
}

resource "opentaco_dependency" "a_subnets_to_b" {
  from_state_id = "org/app/A/terraform.tfstate"
  from_output   = "subnet_ids"

  to_state_id   = "org/app/B/terraform.tfstate"
  to_input      = "subnet_ids"
}
```


---

Implementation Notes (by agent)

- Provider: implemented `opentaco_dependency` in `providers/terraform/opentaco/resources/dependency_resource.go` with deterministic `id`. Computed fields are initialized to known values on create/update (status=`unknown`, digests empty, timestamps null) so Terraform apply never leaves unknowns. The service updates digests/timestamps on state writes.
- Service hooks: added `internal/deps` with:
  - `UpdateGraphOnWrite` — batched, locked read/modify/write of `__opentaco_system_state` on every state write (HTTP backend, S3‑compat, management upload).
  - `ComputeStateStatus` — computes state red/yellow/green and returns incoming edge details.
  - Wired hooks in `internal/backend/handler.go`, `internal/s3compat/handler.go`, `internal/state/handler.go` (fire‑and‑forget; non‑blocking).
- API: new `GET /v1/states/:id/status` route registered in `internal/api/routes.go`.
- CLI: `taco state status [<id> | --prefix <pfx>]` with friendly colors: up to date (green), needs re‑apply (red), might need re‑apply (yellow). No args (or `--prefix /`) shows all states. `-o json` returns raw statuses.
- Example: `examples/dependencies/` (A→B→C). A’s `db_url` includes `timestamp()` to simulate changes on every apply.
- Docs: updated README, AGENTS.md, and Mintlify (`docs/`) to explain graph storage, hashing, hooks, status semantics, and usage.
