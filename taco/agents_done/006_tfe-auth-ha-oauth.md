# TFE Authentication & High Availability OAuth Integration

**Status**: ✅ **COMPLETED**  
**Date**: September 2025  
**Components**: TFE API compatibility, JWT signing, OAuth state management, HA deployment support

## Overview

Implemented comprehensive authentication system supporting both OpenTaco's native JWT-based auth and Terraform Enterprise (TFE) API compatibility, with full high availability (HA) deployment support.

## Key Components Delivered

### 🔑 **Dual Token System**
- **JWT Tokens**: Stateless, cryptographically verified Bearer tokens for OpenTaco API
- **Opaque API Tokens**: Database-backed tokens for TFE API compatibility (`otc_tfe_*` prefix)
- **Unified Issuance**: Both token types issued simultaneously during authentication

```go
// Example response includes both token types
{
    "access_token": "eyJ...",     // JWT (signed with PEM key)
    "token": "otc_tfe_abc123",    // Opaque (stored in S3/memory)
    "token_type": "Bearer",
    "expires_in": 3600
}
```

### 🏢 **High Availability JWT Signing**
- **Shared PEM Keys**: Ed25519 private keys distributed across all instances
- **Stateless Verification**: Any instance can verify JWTs from any other instance
- **Key Rotation Support**: Gradual rollout via `OPENTACO_TOKENS_KID` versioning
- **JWKS Endpoint**: Standards-compliant `/oidc/jwks.json` for public key distribution

**Environment Variables:**
```bash
export OPENTACO_TOKENS_PRIVATE_KEY_PEM_PATH="/etc/keys/jwt-key.pem"
export OPENTACO_TOKENS_KID="v1"
export OPENTACO_TOKENS_ACCESS_TTL="1h"
export OPENTACO_TOKENS_REFRESH_TTL="720h"
```

### 🔒 **OAuth State Encryption**
- **Stateless OAuth Flow**: Session data encrypted in URL state parameter
- **AES-256-GCM Encryption**: Secure, authenticated encryption for session data
- **HA Compatible**: No server-side session storage required
- **Load Balancer Friendly**: No session stickiness needed

**Environment Variables:**
```bash
export OPENTACO_OAUTH_STATE_KEY="your-32+-character-secure-random-key"
```

### 🌐 **OIDC Provider Integration**
- **Generic OIDC Support**: Works with Auth0, Okta, WorkOS, Azure AD, etc.
- **PKCE Flow**: Secure authorization code flow with proof key
- **Group/Role Mapping**: Extracts user groups from ID tokens
- **Email Attribution**: Preserves user email for audit trails

**Environment Variables:**
```bash
export OPENTACO_AUTH_ISSUER="https://your-oidc-provider.com"
export OPENTACO_AUTH_CLIENT_ID="your-client-id"
export OPENTACO_AUTH_CLIENT_SECRET="your-client-secret"
```

### 📡 **TFE API Compatibility Layer**
- **Opaque Token Storage**: S3-backed token persistence with memory fallback
- **Token Lifecycle**: Creation, verification, revocation, last-used tracking
- **TFE Endpoints**: Compatible with Terraform Enterprise API patterns
- **Security Model**: Separate from JWT tokens, database-backed validation

## New API Endpoints

### **Authentication Flow**
- `GET /v1/auth/config` - Server OIDC configuration for CLI discovery
- `POST /v1/auth/exchange` - OIDC ID token → OpenTaco access/refresh tokens
- `POST /v1/auth/token` - Refresh token rotation
- `GET /v1/auth/me` - Current user info from Bearer token

### **OAuth Integration**
- `GET /oauth/authorize` - OAuth authorization endpoint
- `POST /oauth/token` - OAuth token exchange (PKCE)
- `GET /oauth/oidc-callback` - OIDC provider callback handler

### **Key Distribution**
- `GET /oidc/jwks.json` - JWT public key set (JWKS)

### **Token Management**
- `POST /v1/auth/issue-s3-creds` - Stateless STS credential issuance

## Implementation Details

### **File Structure**
```
internal/
├── auth/
│   ├── handler.go      # Authentication HTTP handlers
│   ├── jwt.go          # JWT signing/verification with Ed25519
│   ├── apitokens.go    # Opaque token management (TFE compat)
│   ├── terraform.go    # Terraform CLI OAuth flow
│   └── utils.go        # AES-GCM encryption utilities
├── oidc/
│   ├── oidc.go         # OIDC client configuration
│   └── verifier.go     # ID token verification
├── middleware/
│   └── auth.go         # Authentication middleware
└── rbac/
    └── rbac.go         # Role-based access control
```

### **Security Features**
- **Ephemeral Keys**: Auto-generated Ed25519 keys for development
- **Production Key Loading**: PEM file loading with error handling  
- **Token Scope Validation**: Audience and scope claim verification
- **Secure Defaults**: Strong encryption, short token lifetimes
- **Audit Trails**: Last-used timestamps, creation tracking

### **CLI Integration**
New CLI commands for authentication:
- `taco login` - PKCE-based authentication flow
- `taco logout` - Token revocation
- `taco whoami` - Current user information
- `taco creds --json` - S3 credential helper output

## High Availability Considerations

### **Critical Shared Secrets**
All instances MUST share identical values:
```bash
OPENTACO_TOKENS_PRIVATE_KEY_PEM_PATH="/etc/keys/same-key.pem"
OPENTACO_TOKENS_KID="v1"
OPENTACO_OAUTH_STATE_KEY="same-32-char-key-across-instances"
```

### **Stateless Design**
- **JWT Verification**: No database lookup required
- **OAuth Sessions**: Encrypted in URL parameters
- **Opaque Tokens**: S3-backed for natural HA support
- **Load Balancer Ready**: No session affinity requirements

### **Deployment Patterns**
- **Kubernetes**: Secrets mounted as volumes
- **Docker Compose**: Shared secret files
- **Cloud Services**: Integration with AWS Secrets Manager, HashiCorp Vault
- **Development**: Auto-generated ephemeral keys

## Configuration Examples

### **Production with Auth0**
```bash
# JWT Signing (shared across instances)
export OPENTACO_TOKENS_PRIVATE_KEY_PEM_PATH="/etc/keys/jwt-key.pem"
export OPENTACO_TOKENS_KID="v1"

# OAuth State Encryption (shared across instances)  
export OPENTACO_OAUTH_STATE_KEY="$(openssl rand -base64 32)"

# Public URL for redirects
export OPENTACO_PUBLIC_BASE_URL="https://opentaco.company.com"

# Auth0 OIDC Configuration
export OPENTACO_AUTH_ISSUER="https://company.us.auth0.com"
export OPENTACO_AUTH_CLIENT_ID="your-auth0-client-id"
export OPENTACO_AUTH_CLIENT_SECRET="your-auth0-client-secret"
```

### **Key Generation**
```bash
# Generate Ed25519 key pair for JWT signing
openssl genpkey -algorithm Ed25519 -out opentaco-jwt-key.pem

# Generate OAuth state encryption key
openssl rand -base64 32
```

## Benefits Delivered

### **Enterprise Compatibility**
- **Terraform Enterprise API**: Full TFE endpoint compatibility
- **Existing Workflows**: Drop-in replacement for TFE authentication
- **Token Formats**: Support for both modern JWT and legacy opaque tokens

### **Operational Excellence**
- **Zero-Downtime Deployments**: Stateless authentication allows rolling updates
- **Horizontal Scaling**: No authentication bottlenecks
- **Key Rotation**: Gradual key rollout with zero service interruption
- **Monitoring Ready**: Structured logs and error handling

### **Security Posture**
- **Industry Standards**: JWT, OIDC, PKCE compliance
- **Defense in Depth**: Multiple validation layers
- **Principle of Least Privilege**: Scope-based access control
- **Audit Ready**: Comprehensive request/response logging

## Testing & Validation

### **Integration Tests**
- OAuth flow end-to-end validation
- JWT signing/verification across instances
- Token lifecycle management
- RBAC permission enforcement

### **Security Testing**
- PKCE code challenge validation
- State parameter tampering protection
- Token expiration enforcement
- Cross-site request forgery (CSRF) protection

## Documentation Updates

Updated comprehensive documentation in `docs/`:
- `cloud-backend.md` - HA deployment patterns
- `troubleshooting.md` - Authentication debugging
- Added configuration examples for major OIDC providers

## Migration Path

For existing deployments:
1. **Development**: No changes needed (auto-generates ephemeral keys)
2. **Production**: Add PEM key generation to deployment pipeline
3. **HA Environments**: Distribute shared secrets via your secret management system
4. **CLI Users**: Run `taco login` to authenticate with new system

## Related Work

This builds upon:
- **002_auth-oidc-rbac.md** - Initial RBAC framework
- **003_s3-compat-backend-auth.md** - S3 compatibility layer
- Extends authentication to support both modern and legacy Terraform workflows

---

**Impact**: Enables enterprise deployment of OpenTaco with industry-standard authentication, full HA support, and seamless Terraform Enterprise workflow compatibility.
