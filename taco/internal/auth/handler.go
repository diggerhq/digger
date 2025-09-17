package auth

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"

	"github.com/diggerhq/digger/opentaco/internal/oidc"
	"github.com/diggerhq/digger/opentaco/internal/sts"
	"github.com/labstack/echo/v4"
)

// Handler provides auth-related HTTP handlers.
type Handler struct{
    signer *Signer
    sts    sts.Issuer
    oidcV  oidc.Verifier
    apiTokens *APITokenManager
}

func NewHandlerFromEnv() *Handler {
    signer, _ := NewSignerFromEnv()
    issuer, _ := sts.NewStatelessIssuerFromEnv()
    verifier, _ := oidc.NewFromEnv()
    return &Handler{signer: signer, sts: issuer, oidcV: verifier}
}

func NewHandler(s *Signer, stsi sts.Issuer, ver oidc.Verifier) *Handler {
    return &Handler{signer: s, sts: stsi, oidcV: ver}
}

// GetSigner returns the signer for use in other packages
func (h *Handler) GetSigner() *Signer {
    return h.signer
}

// SetAPITokenManager wires the opaque API token manager for Terraform CLI tokens
func (h *Handler) SetAPITokenManager(m *APITokenManager) {
    h.apiTokens = m
}

// Exchange handles POST /v1/auth/exchange
// Request: {"id_token":"..."}
// Response: {"access_token":"...","refresh_token":"...","expires_in":3600,"token_type":"Bearer"}
func (h *Handler) Exchange(c echo.Context) error {
    var req struct{ IDToken string `json:"id_token"` }
    if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil || req.IDToken == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error":"invalid_request"})
    }
    if h.signer == nil || h.oidcV == nil {
        return c.JSON(http.StatusNotImplemented, map[string]string{
            "error":   "not_implemented",
            "message": "Milestone 1 dummy endpoint",
            "hint":    "This route will be implemented in a later milestone.",
        })
    }
    sub, groups, err := h.oidcV.VerifyIDToken(req.IDToken)
    if err != nil { return c.JSON(http.StatusUnauthorized, map[string]string{"error":"invalid_id_token"}) }
    
    // Extract email from ID token if available
    email := extractEmailFromIDToken(req.IDToken)
    
    access, exp, err := h.signer.MintAccessWithEmail(sub, email, nil, groups, []string{"api","s3"})
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error":"sign_error"}) }
    rid := randomRID()
    refresh, _, err := h.signer.MintRefresh(sub, rid)
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error":"sign_error"}) }
    return c.JSON(http.StatusOK, map[string]any{
        "access_token":  access,
        "refresh_token": refresh,
        "expires_in":    int(exp.Sub(timeNow()).Seconds()),
        "token_type":    "Bearer",
    })
}

// Token handles POST /v1/auth/token (refresh -> new access)
func (h *Handler) Token(c echo.Context) error {
    if h.signer == nil { 
        return c.JSON(http.StatusNotImplemented, map[string]string{
            "error":   "not_implemented",
            "message": "Milestone 1 dummy endpoint",
            "hint":    "This route will be implemented in a later milestone.",
        })
    }
    var req struct{ RefreshToken string `json:"refresh_token"` }
    if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil || req.RefreshToken == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error":"invalid_request"})
    }
    rc, err := h.signer.VerifyRefresh(req.RefreshToken)
    if err != nil { return c.JSON(http.StatusUnauthorized, map[string]string{"error":"invalid_refresh"}) }
    access, exp, err := h.signer.MintAccess(rc.Subject, nil, nil, []string{"api","s3"})
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error":"sign_error"}) }
    // Rotate refresh: for dev, issue new refresh without tracking revocation list
    refresh, _, err := h.signer.MintRefresh(rc.Subject, randomRID())
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error":"sign_error"}) }
    return c.JSON(http.StatusOK, map[string]any{
        "access_token":  access,
        "refresh_token": refresh,
        "expires_in":    int(exp.Sub(timeNow()).Seconds()),
        "token_type":    "Bearer",
    })
}

// IssueS3Creds handles POST /v1/auth/issue-s3-creds (STS)
func (h *Handler) IssueS3Creds(c echo.Context) error {
    if h.signer == nil || h.sts == nil { 
        return c.JSON(http.StatusNotImplemented, map[string]string{
            "error":   "not_implemented",
            "message": "Milestone 1 dummy endpoint",
            "hint":    "This route will be implemented in a later milestone.",
        })
    }
    // Require Authorization: Bearer <access>
    authz := c.Request().Header.Get("Authorization")
    if !strings.HasPrefix(authz, "Bearer ") {
        return c.JSON(http.StatusUnauthorized, map[string]string{"error":"missing_bearer"})
    }
    tokenStr := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
    ac, err := h.signer.VerifyAccess(tokenStr)
    if err != nil { return c.JSON(http.StatusUnauthorized, map[string]string{"error":"invalid_access"}) }
    // Issue stateless creds; SessionToken carries the access token
    akid, sk, st, expUnix, err := h.sts.Issue(ac.Subject, tokenStr)
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error":"sts_issue_failed"}) }
    return c.JSON(http.StatusOK, map[string]any{
        "Version":         1,
        "AccessKeyId":     akid,
        "SecretAccessKey": sk,
        "SessionToken":    st,
        "Expiration":      timeUnixToRFC3339(expUnix),
    })
}

// Me handles GET /v1/auth/me (debug)
func (h *Handler) Me(c echo.Context) error {
    // Echo principal from bearer if present
    authz := c.Request().Header.Get("Authorization")
    if strings.HasPrefix(authz, "Bearer ") && h.signer != nil {
        tokenStr := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
        if ac, err := h.signer.VerifyAccess(tokenStr); err == nil {
            return c.JSON(http.StatusOK, map[string]any{
                "subject": ac.Subject,
                "email":   ac.Email,
                "roles":   ac.Roles,
                "groups":  ac.Groups,
                "scopes":  ac.Scopes,
            })
        }
    }
    // Fallback stub
    return c.JSON(http.StatusOK, map[string]any{
        "subject": "anonymous",
        "email":   "",
        "roles":   []string{},
        "groups":  []string{},
        "scopes":  []string{"api", "s3"},
    })
}

// JWKS handles GET /oidc/jwks.json
func (h *Handler) JWKS(c echo.Context) error {
    if h.signer == nil {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{"error":"jwks_unavailable"})
    }
    return c.JSON(http.StatusOK, h.signer.JWKS())
}


func randomRID() string {
    b := make([]byte, 8)
    _, _ = rand.Read(b)
    return fmt.Sprintf("%x", b)
}

func timeUnixToRFC3339(ts int64) string { return time.Unix(ts, 0).UTC().Format(time.RFC3339) }

var timeNow = func() time.Time { return time.Now() }

// Config exposes server-side OIDC config so the CLI doesn't need flags.
// GET /v1/auth/config
// Response: { issuer, client_id, authorization_endpoint?, token_endpoint?, redirect_uris? }
func (h *Handler) Config(c echo.Context) error {
    issuer := os.Getenv("OPENTACO_AUTH_ISSUER")
    authURL := os.Getenv("OPENTACO_AUTH_AUTH_URL")
    tokenURL := os.Getenv("OPENTACO_AUTH_TOKEN_URL")

    // Provide sensible defaults for common issuers if endpoints are not supplied
    if (authURL == "" || tokenURL == "") && issuer == "https://api.workos.com/user_management" {
        if authURL == "" { authURL = "https://api.workos.com/user_management/authorize" }
        if tokenURL == "" { tokenURL = "https://api.workos.com/user_management/token" }
    }

    cfg := map[string]any{
        "issuer":    issuer,
        "client_id": os.Getenv("OPENTACO_AUTH_CLIENT_ID"),
    }
    if authURL != "" { cfg["authorization_endpoint"] = authURL }
    if tokenURL != "" { cfg["token_endpoint"] = tokenURL }
    // Provide a default loopback redirect for PKCE
    cfg["redirect_uris"] = []string{"http://127.0.0.1:8585/callback"}
    return c.JSON(http.StatusOK, cfg)
}

// extractEmailFromIDToken extracts email from JWT ID token payload
func extractEmailFromIDToken(idToken string) string {
    // Split JWT token into parts
    parts := strings.Split(idToken, ".")
    if len(parts) != 3 {
        return ""
    }
    
    // Decode the payload (second part)
    payload := parts[1]
    // Add padding if needed
    if len(payload)%4 != 0 {
        payload += strings.Repeat("=", 4-len(payload)%4)
    }
    
    data, err := base64.URLEncoding.DecodeString(payload)
    if err != nil {
        return ""
    }
    
    var claims map[string]interface{}
    if err := json.Unmarshal(data, &claims); err != nil {
        return ""
    }
    
    // Try to extract email from various possible fields
    if email, ok := claims["email"].(string); ok {
        return email
    }
    if email, ok := claims["preferred_username"].(string); ok {
        return email
    }
    if email, ok := claims["sub"].(string); ok && strings.Contains(email, "@") {
        return email
    }
    
    return ""
}
