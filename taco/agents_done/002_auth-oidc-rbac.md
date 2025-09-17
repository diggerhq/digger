# OpenTaco — Layer-0 State + Auth/STS (Final Agent Spec)

## Purpose (why we’re building this)

OpenTaco is a self-hostable companion to Terraform/OpenTofu focused on **Layer-0: state control**. It manages state files and access to them, exposes a JSON **management API**, and a **state backend** that Terraform can talk to. We add **one identity plane** (OIDC login) and an **STS** that mints **short-lived, auto-refreshing** credentials for the Terraform **S3 backend**, so **RBAC** is enforced uniformly across both the API and state operations.

* **AuthN:** OIDC only (Authorization Code + PKCE; Device Code optional).
* **AuthZ:** OpenTaco-local **RBAC** (roles & permissions).
* **State backend:** Terraform `backend "s3"` pointed at our **S3-compatible endpoint**; credentials come from a **single global AWS profile** named `opentaco-state-backend`, via `credential_process = taco creds --json`.

---

## High-level flows

1. **Login**: `taco login` (OIDC + PKCE) → server verifies ID token → server issues **OpenTaco access/refresh** tokens.
2. **STS**: Terraform needs creds; AWS SDK runs `credential_process` → `taco creds --json` → server returns **AccessKeyId / SecretAccessKey / SessionToken / Expiration** (15m).
3. **S3-compat requests**: Terraform calls our endpoint; we verify **SigV4** with the issued secret, require the **SessionToken** (our JWT with `aud:"s3"`), then enforce **RBAC** for read/write/lock.
4. **Management API**: CLI / provider call JSON endpoints with `Authorization: Bearer <OpenTaco access token>`; server verifies & applies **RBAC**.

---

## Directory structure (what & why, all under `opentaco/`)

```
opentaco/
├─ README.md                         # quickstart & pointers to docs
├─ Makefile                          # build/lint/test/run targets
├─ .golangci.yml                     # baseline linters
├─ docs/
│  ├─ final_spec_state_auth_sts.md   # ← this document
│  ├─ backend_profile_guide.md       # user-facing how-to (profile + backend block)
│  └─ auth_config_examples.md        # issuer-specific samples (Okta/WorkOS/Keycloak/etc.)
├─ cmd/
│  ├─ opentacosvc/                   # service main (Echo server)
│  └─ taco/                          # CLI main (Cobra: login, creds, state cmds)
├─ internal/
│  ├─ api/                           # JSON management API handlers (CRUD states, locks, upload/download)
│  ├─ s3compat/                      # S3-compatible HTTP handlers (GET/PUT/DELETE, lockfile)
│  ├─ backend/                       # Terraform HTTP backend proxy (GET/POST/PUT/LOCK/UNLOCK) — keep supported
│  ├─ storage/                       # S3 adapter (real bucket), layout helpers & lockfile ops
│  ├─ oidc/                          # OIDC RP (PKCE + Device Code helper), issuer client
│  ├─ auth/                          # OpenTaco JWT mint/verify, JWKS, refresh
│  ├─ sts/                           # Short-lived S3 creds issuance & key derivation (stateless or stateful)
│  ├─ rbac/                          # Roles, permissions, group→role mapping, checker
│  ├─ middleware/                    # AuthN/AuthZ middlewares for API & S3 paths
│  └─ observability/                 # healthz/readyz, metrics, logging adapters
├─ pkg/
│  └─ sdk/                           # typed client used by CLI & provider (auth header handling)
└─ providers/
   └─ terraform/
      └─ opentaco/                   # terraform provider module (uses pkg/sdk)
         ├─ provider/
         ├─ resources/
         ├─ datasources/
         └─ examples/
```

**Why this shape?**

* Clean separation of *protocol surfaces* (`api/`, `s3compat/`, `backend/`) from *cross-cutting concerns* (`auth/`, `rbac/`, `sts/`).
* Storage is pluggable and remains “bucket-only” (S3 as the sole stateful system).
* CLI and provider share a single typed SDK.

---

## Tooling & versions (pin these)

* **Go**: 1.25 with `toolchain go1.25.0`.
* **HTTP**: `github.com/labstack/echo/v4` (Echo) with Recover, RequestID, Gzip, Secure, BodyLimit("10M"), server timeouts (read 10s / write 30s / idle 60s).
* **Logging**: `go.uber.org/zap`.
* **CLI**: `github.com/spf13/cobra`.
* **JWT**: `github.com/golang-jwt/jwt/v5`.
* **OIDC**: `github.com/coreos/go-oidc/v3/oidc` + `golang.org/x/oauth2`.
* **Functional helpers**: `github.com/samber/lo` (for mapping/filtering where it helps clarity).
* **Provider**: `github.com/hashicorp/terraform-plugin-framework` (+ testing/log).
* **Lint**: `golangci-lint` (enable: gofumpt, govet, staticcheck, errcheck, ineffassign, prealloc, gocyclo).

---

## Configuration (single-tenant v1)

Create `opentaco/configs/auth.yaml` (env-overridable):

```yaml
server:
  public_base_url: "https://cloud.opentaco.dev"
  cookie_domain: "cloud.opentaco.dev"     # reserved for web later
auth:
  issuer: "https://YOUR_OIDC_ISSUER"      # WorkOS / Okta / Entra / Keycloak / ZITADEL / Auth0
  client_id: "..."
  client_secret: "..."
  redirect_uris:
    - "http://127.0.0.1:8585/callback"    # CLI loopback (PKCE)
  device_code_enabled: true               # if issuer supports RFC 8628
tokens:
  alg: "EdDSA"                            # or RS256
  private_key_pem_path: "./.secrets/opentaco_ed25519.pem"
  access_ttl: "1h"
  refresh_ttl: "720h"                     # 30d
sts:
  kid: "k1"
  hmac_secrets:
    k1: "base64url-hmac-key"              # for stateless STS secret derivation
  ttl: "15m"
rbac:
  group_role_mapping:
    "tf-admins":   ["admin"]
    "tf-writers":  ["state_writer"]
    "tf-readers":  ["state_reader"]
  allow_prefixes:
    - "org/"                              # default scope for non-admins
```

**Environment variable prefix:** `OPENTACO_` (not OTACO). Common overrides you should support:

* `OPENTACO_AUTH_ISSUER`, `OPENTACO_AUTH_CLIENT_ID`, `OPENTACO_AUTH_CLIENT_SECRET`
* `OPENTACO_PUBLIC_BASE_URL`, `OPENTACO_TOKENS_ACCESS_TTL`, `OPENTACO_TOKENS_REFRESH_TTL`
* `OPENTACO_STS_TTL`, `OPENTACO_STS_KID`, `OPENTACO_STS_HMAC_<KID>`
* `OPENTACO_RBAC_GROUP_ROLE_MAPPING` (or a file path var)
* `OPENTACO_API_BASE_URL` (for the CLI to know where to call)

---

## Surfaces to implement / preserve

### 1) JSON Management API (`/v1`, bearer-protected)

Keep your existing endpoints (CRUD of states, manual lock/unlock, upload/download). Guard them with **AuthN (JWT)** and **RBAC** middleware:

* `POST   /v1/states`
* `GET    /v1/states?prefix=<string>`
* `GET    /v1/states/:id`
* `DELETE /v1/states/:id`
* `GET    /v1/states/:id/download`
* `POST   /v1/states/:id/upload[?if_locked_by=<uuid>]`
* `POST   /v1/states/:id:lock`
* `DELETE /v1/states/:id:unlock`

**RBAC mapping**:

* `state.read` → meta/download
* `state.write` → upload/delete
* `state.lock` → lock/unlock

### 2) S3-compatible endpoint (primary for Terraform)

Continue exposing S3 operations over HTTP and proxying to your real S3 bucket **OR** implementing storage directly—your choice. Add **SigV4 verification** + **SessionToken** enforcement:

* Verify `Authorization` (SigV4) using the **SecretAccessKey** derived from **AccessKeyId**.
* Require `X-Amz-Security-Token` (our **OpenTaco access token** with `aud:"s3"`; may also include scoped prefixes).
* Enforce **RBAC**:

  * GET/HEAD on object → `state.read`
  * PUT/POST on object → `state.write`
  * Lockfile ops (`*.tflock` or `terraform.tfstate.lock`) → `state.lock`

### 3) Terraform HTTP backend proxy (secondary; keep working)

Continue to support `/v1/backend/:id` with `GET`, `POST/PUT`, `LOCK`, `UNLOCK`. This remains handy for users who prefer the HTTP backend.

---

## Auth & tokens (server)

### Endpoints

* `POST /v1/auth/exchange`
  **Request**: `{"id_token":"<OIDC_ID_TOKEN>"}`
  **Response**:

  ```json
  {"access_token":"...","refresh_token":"...","expires_in":3600,"token_type":"Bearer"}
  ```
* `POST /v1/auth/token` (refresh → new access)
* `POST /v1/auth/issue-s3-creds` (STS; see below)
* `GET  /oidc/jwks.json` (publish signer public keys)
* `GET  /v1/auth/me` (debug; echo subject/roles/scopes)

### OpenTaco JWTs

* **Access**: 1h TTL; claims `{iss, sub, aud:["api","s3"], org, roles, groups, scopes, iat, exp, kid}`.
* **Refresh**: 30d TTL; rotating (store rotation IDs minimally).
* **JWKS**: rotate signing keys; keep last N in the set.

---

## STS (short-lived S3 credentials)

### Response shape (AWS Process Credentials JSON)

```json
{
  "Version": 1,
  "AccessKeyId": "OTC.k1.ABCD1234",
  "SecretAccessKey": "base64-secret-derived",
  "SessionToken": "OpenTacoAccessJWTorSTSJWT",
  "Expiration": "2025-08-26T15:45:00Z"
}
```

### Stateless issuance (recommended to start)

* `AccessKeyId = "OTC.<kid>.<session_id>"` (short random).
* `SecretAccessKey = HMAC(hmac_secrets[kid], session_id)`.
* `SessionToken =` the **OpenTaco access token** (must include `aud:"s3"` and optional prefix scopes).
* TTL: default 15m (configurable).

**S3 side verification**

1. Parse `AccessKeyId`, extract `<kid>` and `<session_id>`.
2. Recompute `SecretAccessKey` using HMAC; verify SigV4.
3. Verify `SessionToken` as a valid OpenTaco JWT; assert `aud:"s3"`, `exp` not expired.
4. Build principal (user id + roles) from token; run **RBAC** for the attempted operation.

*(If you prefer immediate revocation, switch to a stateful STS with Redis/Postgres later; the API remains the same.)*

---

## RBAC model (minimal & explicit)

* **Roles → permissions**

  * `admin`: `state.read`, `state.write`, `state.lock` on all keys
  * `state_writer`: `state.read`, `state.write`, `state.lock` on allowed prefixes
  * `state_reader`: `state.read` on allowed prefixes

* **Subjects → roles**
  From OIDC `groups[]` claim via `rbac.group_role_mapping`.

* **Checker**
  Pure function: `rbac.Can(principal, action, stateKey) bool`.
  Use it in both **API** and **S3** paths.

---

## CLI (user-facing commands to add/confirm)

* `taco login`

  * OIDC Auth Code + PKCE (loopback `127.0.0.1:<port>/callback`).
  * Fallback: Device Code if issuer supports it.
  * Stores **refresh token** at `~/.config/opentaco/credentials.json` (per host or per base URL).

* `taco creds --json`

  * Ensures valid OpenTaco access token (refresh as needed).
  * Calls `/v1/auth/issue-s3-creds`.
  * Prints AWS Process Credentials JSON **exactly** as expected by `credential_process`.

* (Optional) `taco whoami` → calls `/v1/auth/me` and prints roles/scopes.

**CLI env**: respect `OPENTACO_API_BASE_URL` to locate the service.

---

## State backend (Terraform) — **global profile is the only path**

**User instruction to document:**

1. **Create profile** in `~/.aws/config` (or Windows equivalent):

   ```
   [profile opentaco-state-backend]
   region = auto
   credential_process = taco creds --json
   ```

   (Also recommend `AWS_SDK_LOAD_CONFIG=1`.)

2. **Backend block**:

   ```hcl
   terraform {
     backend "s3" {
       bucket  = "opentaco"
       key     = "org/app/prod/terraform.tfstate"
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
   ```

3. **Run**:

   ```bash
   taco login
   terraform init -reconfigure
   terraform apply
   ```

**Notes**

* With `credential_process`, the AWS SDK will **auto-refresh** creds during long runs; no custom wrapper needed.
* Keep accepting both `terraform.tfstate.lock` and `.tflock` filenames for max compatibility.

---

## Observability & hardening

* Endpoints: `/healthz`, `/readyz`, `/metrics` (Prometheus), `/debug/pprof` (dev).
* Log each API & S3 op with `(org, user, action, stateKey, allow/deny, reason)`.
* Timeouts: read=10s, write=30s, idle=60s.
* TLS everywhere (terminate at proxy if needed).
* Key rotation: maintain JWKS with current + previous keys; mark current via `kid`.
* Security headers on API; large body caps on upload.

---

## Acceptance criteria (Definition of Done)

* **Auth**

  * `taco login` completes (OIDC) and persists refresh token.
  * `/v1/auth/exchange`, `/v1/auth/token`, `/oidc/jwks.json` function end-to-end.

* **STS**

  * `/v1/auth/issue-s3-creds` returns valid AWS Process Credentials JSON.
  * `taco creds --json` exits 0 and prints that JSON.

* **State backend (S3-compat)**

  * Terraform with the **global profile** (`opentaco-state-backend`) can `init/plan/apply` against our endpoint; native lockfile works.
  * Long `apply` auto-refreshes creds (validated by making STS TTL short, e.g., 2m, and observing refresh calls).
  * RBAC enforced: a `state_reader` cannot write/lock; a `state_writer` can; denial returns a clear 403 with reason.

* **JSON Management API**

  * All routes require bearer token and pass RBAC.
  * CRUD/lock/upload/download work against the real S3 bucket.

* **Provider**

  * Configures against `OPENTACO_API_BASE_URL`, obtains token from CLI credentials file or `OPENTACO_ACCESS_TOKEN`, and successfully reads state meta (authz respected).

* **Docs**

  * `docs/backend_profile_guide.md` explains profile creation, backend block, and the `taco login` → `terraform init` flow.
  * `docs/auth_config_examples.md` includes issuer samples (e.g., WorkOS/Okta/Keycloak) using the same OIDC fields.

* **Quality**

  * `golangci-lint` passes; unit tests for JWT, STS derivation, SigV4 verify (happy path + bad signature), and RBAC checker.
  * Integration test: local Terraform project uses the backend to read/write a test state key through our endpoint with locks and checks RBAC deny/allow.

---

## Implementation tips & edge cases

* **SigV4 verification**: compare signatures **constant-time**; ensure canonical request building matches AWS spec (headers order, date, payload hash).
* **SessionToken**: always require `X-Amz-Security-Token`; reject missing/expired tokens even if SigV4 matches.
* **Region handling**: accept `auto` and any value; use it only for SigV4 scope string (don’t be strict).
* **Key layout**: preserve users’ existing S3 layouts; don’t force renames. If you normalize, document precisely (e.g., `<prefix>/<id>/terraform.tfstate`).
* **Lock semantics**: tolerate both native `.tflock` and `terraform.tfstate.lock`.
* **Provider auth**: simplest path is to have provider read `~/.config/opentaco/credentials.json` (or `OPENTACO_ACCESS_TOKEN`) and send bearer headers via `pkg/sdk`.
* **Multi-tenant future**: keep the OIDC client abstraction & RBAC clean so adding tenant routing later is only config + org resolution.

---

## Deliverables checklist (agents)

* [ ] New packages: `internal/oidc`, `internal/auth`, `internal/sts`, `internal/rbac`, `internal/middleware`.
* [ ] New endpoints wired in service main; middlewares applied on API & S3 routes.
* [ ] CLI: `login`, `creds --json`, (`whoami` optional).
* [ ] Profile docs and examples using **`opentaco-state-backend`** only.
* [ ] Tests: unit (JWT/STS/RBAC/SigV4), integration (Terraform end-to-end), plus linters and health checks.
* [ ] Minimal docs under `opentaco/docs/` as described.

---

**That’s the full brief.** It’s one codepath (OIDC-only), a single global profile for Terraform (`opentaco-state-backend`), and short-lived creds that unify auth across API and state.
