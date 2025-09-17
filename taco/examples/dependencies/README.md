Dependencies Demo (A → B → C)

This example demonstrates output-level dependencies using the `opentaco_dependency` Terraform resource and how OpenTaco computes unit status (red/yellow/green) without any external DB. All dependency metadata lives in the normal Terraform state of a dedicated workspace `__opentaco_system`.

- System workspace (`system/`) declares two edges:
  - A → B on `db_url`
  - B → C on `image_tag`
- Workspaces `a/`, `b/`, `c/` are trivial Terraform projects that write state via the OpenTaco HTTP backend.
- On writes:
  - Source refresh (outgoing): updates `in_digest` (hash of source output) and sets status `pending|ok|unknown`.
  - Target acknowledge (incoming): copies `out_digest ← in_digest`, sets status `ok`.

Status semantics
- red: target has any incoming edge with status `pending` (new upstream change not yet acknowledged)
- yellow: target has no incoming `pending`, but an upstream (transitively) is red
- green: neither red nor yellow

Prereqs
- Go 1.25+
- Terraform CLI

Quick start
1) Build and run the service

```
make build
./opentacosvc -auth-disable -storage memory
# Service on http://localhost:8080
```

2) System workspace — declare states and dependencies

```
cd examples/dependencies/system
terraform init
terraform apply -auto-approve
```
This creates the `__opentaco_system` graph, registers three units (A, B, C), and two edges: `A→B (db_url)` and `B→C (image_tag)`.

3) Apply all three workspaces (write some initial state)

Create the three states by applying their Terraform projects so they exist and will appear in listings:

```
# A
cd examples/dependencies/a
terraform init
terraform apply -auto-approve

# B
cd ../b
terraform init
terraform apply -auto-approve

# C
cd ../c
terraform init
terraform apply -auto-approve
```

4) Re-apply A (simulate a new upstream change)

A's `db_url` output includes a timestamp so it changes on every apply. Re-apply A to simulate a fresh upstream change and mark `A→B` as pending:

```
cd ../a
terraform apply -auto-approve
```

5) Check status for B and C

```
cd ../../..
./taco unit status --prefix org/app/
```
Expected:
- B is red with 1 pending edge (A→B)
- C is yellow (downstream of red B)

6) Apply B (acknowledges the incoming edge and turns green)

```
cd examples/dependencies/b
terraform apply -auto-approve
```

7) Check status again

```
cd ../../..
./taco unit status --prefix org/app/
```
Expected:
- B is green (ack complete)
- C is still yellow until C writes once

8) Apply C (acknowledge B→C)

```
cd examples/dependencies/c
terraform apply -auto-approve
```

9) Final status

```
cd ../../..
./taco unit status --prefix org/app/
# All green
```

Notes
- IDs are logical (e.g., `org/app/A`), the backend stores objects at `<prefix>/<id>/terraform.tfstate`.
- Only digests/timestamps are stored in the graph; no plaintext output values are persisted.
- You can customize the server URL by editing `backend` blocks if not running on localhost.
