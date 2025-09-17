# Task: S3‑Compatible Backend with OIDC + STS + SigV4 + RBAC

## Purpose
Expose an S3‑compatible HTTP surface that Terraform’s `backend "s3"` can use against the OpenTaco service. Authenticate requests via OpenTaco‑minted short‑lived STS credentials, verify AWS SigV4, require the OpenTaco access token as the session token, and enforce RBAC. Keep the existing HTTP backend proxy (`/v1/backend/*`) fully supported.

## Scope
- Add an S3‑compatible endpoint under `opentaco/internal/s3compat` wired at `GET/PUT/POST/DELETE/HEAD` to prefix `/s3`.
- Verify SigV4 using recomputation with `aws-sdk-go-v2/aws/signer/v4` and derived STS secrets.
- Require `X-Amz-Security-Token` (OpenTaco access JWT) and verify with audience includes `"s3"`.
- Map object requests to existing storage semantics (bucket‑only layout; in-memory fallback still supported):
  - Object key: `<prefix>/<state-id>/terraform.tfstate`.
  - Lock file: `<prefix>/<state-id>/terraform.tfstate.lock` and `*.tflock`.
- Enforce RBAC via `rbac.Can(principal, action, stateKey)` for object read/write/lock.
- Keep auth ON by default. Single bypass switch remains `-auth-disable` (dev only).

## Non‑Goals
- No multi‑tenant/organizations (single tenant only).
- No server‑side S3 persistence beyond the existing storage adapters.
- No S3 features outside what Terraform backend needs (no multipart, ACLs, list buckets, etc.).

## API Shape & Routing
- Prefix: `/s3` (path‑style addressing).
  - Example paths Terraform will hit:
    - `GET  /s3/opentaco/org/app/prod/terraform.tfstate`
    - `PUT  /s3/opentaco/org/app/prod/terraform.tfstate`
    - `HEAD /s3/opentaco/org/app/prod/terraform.tfstate`
    - `PUT  /s3/opentaco/org/app/prod/terraform.tfstate.lock` (or `.tflock`)
    - `DELETE /s3/opentaco/org/app/prod/terraform.tfstate.lock` (or `.tflock`)
- We ignore the first path segment as a logical bucket name (Terraform requires one); storage is bucket‑only.

## Security Model
- STS: stateless creds with AccessKeyId format `OTC.<kid>.<session_id>`.
  - `SecretAccessKey = HMAC(hmac_secrets[kid], session_id)` using `OPENTACO_STS_HMAC_<KID>`.
  - `SessionToken` is the OpenTaco access JWT (aud contains `"s3"`).
- Request requirements:
  - `Authorization: AWS4-HMAC-SHA256 Credential=OTC.<kid>.<sid>/<date>/<region>/s3/aws4_request, ...`
  - `X-Amz-Security-Token: <OpenTaco access JWT>`
  - `X-Amz-Date` and signed headers present; payload hash via `X-Amz-Content-Sha256` or computed.
- Verification steps:
  1) Parse AccessKeyId: extract `<kid>` and `<session_id>`.
  2) Derive `SecretAccessKey` via HMAC using configured secret for `<kid>`.
  3) Verify `SessionToken` with OpenTaco JWT verifier (`aud` includes `"s3"`, not expired).
  4) Recompute the SigV4 signature using `aws-sdk-go-v2/aws/signer/v4` with the derived secret, the same request, and the same time/scope; constant‑time compare with provided signature.
  5) Build principal from access token claims and run RBAC.
- Support both header‑based and query‑based SigV4 (presigned URL) if feasible (priority: header‑based; add query later if needed).

## RBAC Mapping
- `state.read` → GET/HEAD object
- `state.write` → PUT/POST object
- `state.lock` → lockfile operations (PUT/DELETE on `terraform.tfstate.lock` or `*.tflock`)
- Principal resolved from access JWT: subject, groups → roles via mapping; call `rbac.Can(principal, action, stateKey)`.

## Storage Mapping
- State object path: `<prefix>/<state-id>/terraform.tfstate`.
- Lockfile: `terraform.tfstate.lock` and `.tflock` (Terraform writes JSON). Parse into `storage.LockInfo {id, who, created, version}`.
- For lock PUT:
  - If JSON lock info is posted to the lock key, run `store.Lock(id, lockInfo)`.
- For lock DELETE:
  - Require lock ID; parse from JSON or headers; run `store.Unlock(id, lockID)`.
- For GET/HEAD lockfile:
  - Return 200 with current lock JSON or 404 if unlocked.

## Implementation Plan
1) Package & Routes
   - Add `internal/s3compat/` with `handler.go` and helpers.
   - Wire routes in `internal/api/routes.go`: `e.Any("/s3/*", s3compat.Router(...))` with explicit methods `GET/PUT/POST/DELETE/HEAD`.
   - Apply `RequireAuth`? Not needed for SigV4 path; auth is the SessionToken + SigV4. Still allow `-auth-disable` to bypass for dev (skip verification and RBAC when disabled).

2) Verifier & Derivation
   - Implement `verifySigV4(c *echo.Context) (principal, stateKey, action, error)`:
     - Parse headers (or query) for SigV4 fields.
     - Derive `SecretAccessKey` from `kid` + `session_id`.
     - Verify `SessionToken` via `internal/auth.Signer.VerifyAccess` (must include `aud:["s3"]`).
     - Recompute signature with `v4.NewSigner().SignHTTP(...)` using derived creds; compare.
   - Map HTTP method+path to action (read/write/lock) and to `stateKey` (strip leading bucket segment and trailing filename to get the state ID).

3) Handlers
   - GET/HEAD on tfstate → `store.Download` (HEAD returns headers only).
   - PUT/POST on tfstate → `store.Upload` (respect lock if present).
   - LOCKFILES:
     - PUT on lock path: parse JSON to `LockInfo`, call `store.Lock`.
     - DELETE on lock path: parse JSON or `If-Match`/header for lock ID, call `store.Unlock`.
     - GET/HEAD lock path: return lock info if present, else 404.

4) RBAC Enforcement
   - After verification, compute `action` and call `rbac.Can(principal, action, stateKey)`; 403 on deny.
   - Keep current `rbac.Can` permissive until a real policy is configured.

5) Observability & Errors
   - On each op, log `(principal, action, stateKey, allow|deny, reason)`.
   - Return 401 for missing/invalid token, 403 for bad signature/denied RBAC, 404 for missing objects.

6) Docs & Examples
   - Add “S3‑compatible backend” page with backend block, profile instructions, and notes on skips.
   - Update troubleshooting with signature/session‑token errors.

7) Keep Existing Behavior Intact
   - No regressions to `/v1/backend/*`, state CRUD APIs, CLI & provider.

## Acceptance Criteria
- Service exposes `/s3` and supports GET/PUT/HEAD/DELETE for tfstate and lockfiles.
- Terraform configured with the global profile (`opentaco-state-backend`) can `init/plan/apply` against `/s3` with auth enforced.
- STS TTL refresh works: long apply triggers multiple `/v1/auth/issue-s3-creds` calls, and SigV4 re‑verification succeeds.
- Missing/invalid session token or signature → 401/403 with clear reason.
- Lock semantics respected: concurrent plan/apply conflicts; lock/unlock via lockfiles behave as expected.
- RBAC hook called (permissive for now); later mapped to real roles/prefixes without changing shapes.
- golangci‑lint passes; no changes outside `opentaco/`.

## Test Scenario (Manual)
1) Build & Run
   - `make clean && make build`
   - `export OPENTACO_AUTH_ISSUER="https://<TENANT>.auth0.com"`
   - `export OPENTACO_AUTH_CLIENT_ID="<CLIENT_ID>"`
   - `export OPENTACO_STS_KID="k1"`
   - `export OPENTACO_STS_HMAC_k1="<BASE64URL_SECRET_32B>"`
   - `export OPENTACO_STS_TTL="2m"`
   - `./opentacosvc -storage memory`

2) Login & STS
   - `./taco login --server http://localhost:8080`
   - `./taco whoami`
   - `./taco creds --json`

3) AWS Profile
   - `~/.aws/config`:
     ```
     [profile opentaco-state-backend]
     region = auto
     credential_process = taco creds --json
     ```
   - `export AWS_SDK_LOAD_CONFIG=1`

4) Terraform Backend (S3)
   - Backend block:
     ```hcl
     terraform {
       backend "s3" {
         bucket  = "opentaco"
         key     = "org/app/prod/terraform.tfstate"
         region  = "auto"
         endpoints = { s3 = "http://localhost:8080/s3" }
         use_path_style              = true
         skip_credentials_validation = true
         skip_metadata_api_check     = true
         skip_region_validation      = true
         skip_requesting_account_id  = true
         use_lockfile                = true
         profile = "opentaco-state-backend"
       }
     }
     ```
   - `terraform init -reconfigure`
   - `terraform apply -auto-approve`

5) Lock Test
   - Run `terraform plan` in two terminals; second should error with lock conflict.

6) STS Refresh
   - With TTL=2m, run `for i in {1..5}; do terraform refresh; sleep 90; done` and observe periodic `/v1/auth/issue-s3-creds` in service logs.

7) Negative Cases
   - Remove/alter `X-Amz-Security-Token` → 401/403.
   - Tamper AccessKeyId/secret → 403.

## Notes & Edge Cases
- Handle header‑based SigV4 first; consider query‑based presign support if Terraform uses it (rare for backend).
- Ensure canonical request matches AWS spec (header order, host, path encoding, payload hash).
- Region in SigV4 scope is accepted as given (we don’t enforce specific regions).
- Path handling must preserve users’ key layout (no forced normalization beyond documented layout).


---

Remarks (implementation notes by agent)

- Added minimal ListObjectsV2 at bucket root (`?list-type=2`) to satisfy Terraform workspace probing.
- For empty/uninitialized state, GET/HEAD now return 404 to prompt initialization (prevents infinite polling loops).
- Lockfiles: support both `terraform.tfstate.lock` and `terraform.tfstate.tflock`. Lock PUT is idempotent; DELETE without explicit ID unlocks using the current lock.
- During PUT of `terraform.tfstate` while a lockfile is present, the server injects the current lock ID to satisfy storage ownership checks expected by Terraform.
- SigV4 verification supports both header-based and presigned URL styles; requires `X-Amz-Security-Token` to carry an OpenTaco access token with `aud` including `s3`.
- CLI fix: `taco state unlock` uses an authed client and accepts lock ID positionally (no `--id` flag).
- Docs updated with S3-compatible backend usage, troubleshooting, and navigation links.

