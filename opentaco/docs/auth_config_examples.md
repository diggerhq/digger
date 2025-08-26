# Auth Config Examples

## Auth0 (Native / PKCE)

In Auth0, create a Native application:
- Allowed Callback URLs: `http://127.0.0.1:8585/callback`
- Enable a database or social connection for sign-in

Server env (OpenTaco):
```bash
export OPENTACO_AUTH_ISSUER="https://<TENANT>.auth0.com"     # or <region>.auth0.com
export OPENTACO_AUTH_CLIENT_ID="<AUTH0_NATIVE_APP_CLIENT_ID>"
./opentacosvc -storage memory
```

CLI login (no flags needed):
```bash
./taco login --server http://localhost:8080
```

## WorkOS (User Management / PKCE)

In WorkOS, create a User Management project with a Native (PKCE) OAuth application:
- Allowed Redirect URLs: `http://127.0.0.1:8585/callback`

Server env (OpenTaco):
```bash
export OPENTACO_AUTH_ISSUER="https://api.workos.com/user_management"
export OPENTACO_AUTH_CLIENT_ID="<WORKOS_CLIENT_ID>"
# Optional if you use a hosted domain like https://auth.example.com
# export OPENTACO_AUTH_AUTH_URL="https://auth.example.com/user_management/authorize"
# export OPENTACO_AUTH_TOKEN_URL="https://auth.example.com/user_management/token"
./opentacosvc -storage memory
```

CLI login:
```bash
./taco login --server http://localhost:8080
```

Tips
- Use `--force-login` to show the hosted login box even with an existing SSO session.
- For Auth0, you can clear the SSO session via `/v2/logout?client_id=...&returnTo=...`.
