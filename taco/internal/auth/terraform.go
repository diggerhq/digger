package auth

import (
    "context"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/mr-tron/base58"
)

const (
    defaultClientID = "terraform-cli"
)

// Server-side config shape
type serverAuthConfig struct {
    Issuer                string   `json:"issuer"`
    ClientID              string   `json:"client_id"`
    ClientSecret          string   `json:"client_secret,omitempty"`
    AuthorizationEndpoint string   `json:"authorization_endpoint"`
    TokenEndpoint         string   `json:"token_endpoint"`
    RedirectURIs          []string `json:"redirect_uris"`
}

// AuthCode represents auth code data for JWT creation
type AuthCode struct {
    ClientID      string   `json:"client_id"`
    RedirectURI   string   `json:"redirect_uri"`
    Subject       string   `json:"subject"`
    Email         string   `json:"email"`
    Groups        []string `json:"groups"`
    CodeChallenge string   `json:"code_challenge"`
    Org           string   `json:"org"`
}

// OAuthSession represents OAuth session data encoded in encrypted state
type OAuthSession struct {
    ClientID            string `json:"client_id"`
    RedirectURI         string `json:"redirect_uri"`
    State               string `json:"state"`
    CodeChallenge       string `json:"code_challenge"`        // Terraform's original challenge
    ServerCodeVerifier  string `json:"server_code_verifier"`  // OpenTaco's code verifier for Okta
    Org                 string `json:"org"`                    // Organization UUID from CLI
}

// TerraformServiceDiscovery handles /.well-known/terraform.json
func (h *Handler) TerraformServiceDiscovery(c echo.Context) error {
    baseURL := getBaseURL(c)
    
    discovery := map[string]interface{}{
        "login.v1": map[string]interface{}{
            "client": defaultClientID,
            "grant_types": []string{"authz_code"},
            "authz": baseURL + "/oauth/authorization", 
            "token": baseURL + "/oauth/token",
            "ports": []int{10000, 10010},
        },
        "tfc.v1": "/api/v2",  // Terraform Cloud instead of Enterprise
    }
    
    return c.JSON(http.StatusOK, discovery)
}

// OAuthAuthorize handles GET /oauth/authorization
func (h *Handler) OAuthAuthorize(c echo.Context) error {
    // Extract OAuth parameters
    clientID := c.QueryParam("client_id")
    redirectURI := c.QueryParam("redirect_uri")
    responseType := c.QueryParam("response_type")
    state := c.QueryParam("state")
    scope := c.QueryParam("scope")
    codeChallenge := c.QueryParam("code_challenge")
    codeChallengeMethod := c.QueryParam("code_challenge_method")
    
    // Validate required parameters (accept any non-empty client_id; Terraform may generate one)
    if clientID == "" {
        return c.String(http.StatusBadRequest, "Invalid client_id")
    }
    if responseType != "code" {
        return c.String(http.StatusBadRequest, "Invalid response_type")
    }
    if redirectURI == "" {
        return c.String(http.StatusBadRequest, "Missing redirect_uri")
    }
    // Validate Terraform CLI redirect URI: loopback only, http, and known port window
    if !isValidTerraformRedirectURI(redirectURI) {
        return c.String(http.StatusBadRequest, "Invalid redirect_uri for Terraform login")
    }
    if codeChallenge == "" || codeChallengeMethod != "S256" {
        return c.String(http.StatusBadRequest, "PKCE required: code_challenge with method S256")
    }
    
    // Store the OAuth request parameters for the callback
    sessionData := map[string]string{
        "client_id":             clientID,
        "redirect_uri":          redirectURI,  
        "state":                 state,
        "scope":                 scope,
        "code_challenge":        codeChallenge,
        "code_challenge_method": codeChallengeMethod,
    }
    
    return h.renderOAuthLoginPage(c, sessionData)
}

// OAuthToken handles POST /oauth/token  
func (h *Handler) OAuthToken(c echo.Context) error {
    if h.signer == nil {
        return c.JSON(http.StatusUnauthorized, map[string]string{
            "error": "unauthorized",
            "message": "OAuth token endpoint requires JWT signer",
        })
    }  
    // Parse form data
    grantType := c.FormValue("grant_type")
    code := c.FormValue("code")
    clientID := c.FormValue("client_id")
    redirectURI := c.FormValue("redirect_uri")
    codeVerifier := c.FormValue("code_verifier")
    
    if grantType != "authorization_code" {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "unsupported_grant_type",
        })
    }
    
    // Get the expected client ID from environment (same as Config endpoint)
    expectedClientID := getenv("OPENTACO_AUTH_CLIENT_ID", defaultClientID)
    
    if clientID != expectedClientID {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "invalid_client",
        })
    }
    
    // Verify the authorization code
    authCode, err := h.verifyAuthCode(code)
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "invalid_grant",
            "error_description": err.Error(),
        })
    }
    
    // Validate PKCE
    if !verifyPKCE(codeVerifier, authCode.CodeChallenge) {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "invalid_grant", 
            "error_description": "PKCE verification failed",
        })
    }
    
    // Validate redirect URI matches
    if redirectURI != authCode.RedirectURI {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "invalid_grant",
            "error_description": "Redirect URI mismatch", 
        })
    }
    
    // No need to delete the code - it's stateless and expires automatically
    
    // Issue access token - allow overriding TTL for Terraform CLI tokens
    // If OPENTACO_TERRAFORM_TOKEN_TTL is set (e.g., "720h"), use it; otherwise default to access TTL
    var accessToken string
    var exp time.Time
    if ttlStr := getenv("OPENTACO_TERRAFORM_TOKEN_TTL", ""); ttlStr != "" {
        if ttl, err := time.ParseDuration(ttlStr); err == nil {
            accessToken, exp, err = h.signer.MintAccessWithOrgAndTTL(
                authCode.Subject,
                authCode.Email,
                nil,
                authCode.Groups,
                []string{"api", "s3"},
                authCode.Org,
                ttl,
            )
        }
    }
    if accessToken == "" {
        accessToken, exp, err = h.signer.MintAccessWithOrg(
            authCode.Subject,
            authCode.Email,
            nil,
            authCode.Groups,
            []string{"api", "s3"},
            authCode.Org,
        )
    }
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": "server_error",
        })
    }
    
    // Issue an opaque API token for TFE compatibility (like real Terraform Cloud)
    if h.apiTokens != nil {
        org := authCode.Org
        if org == "" {
            return echo.NewHTTPError(http.StatusBadRequest, "org_uuid required in token claims")
        }
        
        if opaque, err2 := h.apiTokens.Issue(c.Request().Context(), org, authCode.Subject, authCode.Email, authCode.Groups); err2 == nil {
            // Return opaque token as access_token (matching TFE behavior)
            // Calculate expiration based on TERRAFORM_TOKEN_TTL or default to very long
            var expiresIn int
            if ttlStr := getenv("OPENTACO_TERRAFORM_TOKEN_TTL", ""); ttlStr != "" {
                if ttl, err := time.ParseDuration(ttlStr); err == nil {
                    expiresIn = int(ttl.Seconds())
                } else {
                    expiresIn = 999999999 // Very long if TTL parse fails
                }
            } else {
                expiresIn = 999999999 // Very long by default (like TFE)
            }
            
            response := map[string]interface{}{
                "access_token": opaque,  // Use opaque token as main token
                "token_type":   "Bearer",
                "expires_in":   expiresIn,
                "scope":        "api s3",
            }
            return c.JSON(http.StatusOK, response)
        }
    }
    
    // Fallback: return JWT if opaque token creation fails
    response := map[string]interface{}{
        "access_token": accessToken,
        "token_type":   "Bearer",
        "expires_in":   int(exp.Sub(time.Now()).Seconds()),
        "scope":        "api s3",
    }
    
    return c.JSON(http.StatusOK, response)
}

// OAuthLoginRedirect handles the redirect to OIDC provider
func (h *Handler) OAuthLoginRedirect(c echo.Context) error {
    // Extract session data from query params
    clientID := c.QueryParam("client_id")
    redirectURI := c.QueryParam("redirect_uri") 
    state := c.QueryParam("state")
    terraformCodeChallenge := c.QueryParam("code_challenge")
    org := c.QueryParam("org")
    
    // Get OIDC configuration  
    serverConfig, err := h.getServerAuthConfig()
    if err != nil {
        return c.String(http.StatusInternalServerError, "Failed to get auth configuration")
    }
    
    if serverConfig.Issuer == "" || serverConfig.ClientID == "" {
        return c.String(http.StatusInternalServerError, "OIDC not configured")
    }
    
    // Build OIDC authorization URL
    baseURL := getBaseURL(c)
    callbackURL := baseURL + "/oauth/oidc-callback"
    
    // Generate our own PKCE parameters for OpenTaco -> Okta flow
    serverCodeVerifier := generateCodeVerifier()
    serverCodeChallenge := generateCodeChallenge(serverCodeVerifier)
    
    // Create OAuth session data
    sessionData := &OAuthSession{
        ClientID:            clientID,
        RedirectURI:         redirectURI,
        State:               state,
        CodeChallenge:       terraformCodeChallenge,
        ServerCodeVerifier:  serverCodeVerifier,
        Org:                 org,
    }
    
    // Encrypt the session data into the state parameter
    encryptionKey := getEncryptionKey()
    oauthState, err := encryptOAuthSession(sessionData, encryptionKey)
    if err != nil {
        return c.String(http.StatusInternalServerError, "Failed to create OAuth state")
    }
    
    // Redirect to OIDC provider
    authURL := serverConfig.AuthorizationEndpoint
    if authURL == "" {
        return c.String(http.StatusInternalServerError, "Authorization endpoint not configured")
    }
    
    params := url.Values{}
    params.Set("response_type", "code")
    params.Set("client_id", serverConfig.ClientID) 
    params.Set("redirect_uri", callbackURL)
    params.Set("scope", "openid profile email")
    params.Set("state", oauthState)
    
    // Use our own PKCE parameters for OpenTaco -> Okta flow
    params.Set("code_challenge", serverCodeChallenge)
    params.Set("code_challenge_method", "S256")
    
    redirectURL := authURL + "?" + params.Encode()
    
    // Debug: log the redirect URL (remove in production)
    fmt.Printf("Redirecting to OIDC provider: %s\n", redirectURL)
    
    return c.Redirect(http.StatusFound, redirectURL)
}

// OAuthOIDCCallback handles the callback from external OIDC provider
func (h *Handler) OAuthOIDCCallback(c echo.Context) error {
    code := c.QueryParam("code")
    state := c.QueryParam("state")
    errorCode := c.QueryParam("error")
    
    if errorCode != "" {
        return c.String(http.StatusBadRequest, fmt.Sprintf("OIDC error: %s", errorCode))
    }
    
    if code == "" {
        return c.String(http.StatusBadRequest, "Missing code parameter")
    }
    
    // Decrypt the OAuth session data from the state parameter
    encryptionKey := getEncryptionKey()
    sessionData, err := decryptOAuthSession(state, encryptionKey)
    if err != nil {
        return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid or expired state: %v", err))
    }
    
    // Exchange OIDC code for tokens
    serverConfig, err := h.getServerAuthConfig()
    if err != nil {
        return c.String(http.StatusInternalServerError, "Failed to get auth configuration")
    }
    
    baseURL := getBaseURL(c)
    callbackURL := baseURL + "/oauth/oidc-callback"
    
    // Exchange code for ID token
    tokenResp, err := h.exchangeOIDCCode(serverConfig, code, callbackURL, sessionData.ServerCodeVerifier)
    if err != nil {
        return c.String(http.StatusInternalServerError, fmt.Sprintf("Token exchange failed: %v", err))
    }
    
    if tokenResp.IDToken == "" {
        return c.String(http.StatusInternalServerError, "No ID token received")
    }
    
    // Verify ID token and extract user info
    if h.oidcV == nil {
        return c.String(http.StatusInternalServerError, "OIDC verifier not configured")
    }
    
    subject, groups, err := h.oidcV.VerifyIDToken(tokenResp.IDToken)
    if err != nil {
        return c.String(http.StatusInternalServerError, fmt.Sprintf("ID token verification failed: %v", err))
    }
    
    email := extractEmailFromIDToken(tokenResp.IDToken)
    
    // Ensure user has an org
    log.Printf("[OAuth] About to ensure org for user: %s", subject)
    org, err := h.ensureUserHasOrg(c.Request().Context(), subject, email)
    if err != nil {
        log.Printf("[OAuth] Failed to ensure user org: %v", err)
        return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to ensure user org: %v", err))
    }
    log.Printf("[OAuth] User org ensured: %s", org)
    
    // Create authorization code for Terraform
    authCodeData := &AuthCode{
        ClientID:      sessionData.ClientID,
        RedirectURI:   sessionData.RedirectURI, 
        Subject:       subject,
        Email:         email,
        Groups:        groups,
        CodeChallenge: sessionData.CodeChallenge,
        Org:           org,
    }
    
    log.Printf("[OAuth] Creating auth code with org: %s", org)
    
    authCode, err := h.createAuthCode(authCodeData)
    if err != nil {
        return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create auth code: %v", err))
    }
    
    // Redirect back to Terraform's local callback server
    terraformRedirectURI := sessionData.RedirectURI
    terraformState := sessionData.State
    
    params := url.Values{}
    params.Set("code", authCode)
    params.Set("state", terraformState)
    
    finalRedirectURL := terraformRedirectURI + "?" + params.Encode()
    
    // Debug logging
    fmt.Printf("Making server-side callback to Terraform: %s\n", finalRedirectURL)
    
    // Show success page with JavaScript callback to CLI (best UX)
    return h.renderSuccessPageWithCallback(c, finalRedirectURL)
}

// renderSuccessPageWithCallback shows success page and triggers CLI callback via JavaScript
func (h *Handler) renderSuccessPageWithCallback(c echo.Context, callbackURL string) error {
    html := fmt.Sprintf(`
        <html>
        <head>
            <title>OpenTaco - Authentication Complete</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; background: #f8f9fa; }
                .success-container { text-align: center; background: white; border: 1px solid #ddd; padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
                .brand-header { font-size: 32px; margin-bottom: 30px; font-weight: bold; }
                .success-title { color: #28a745; margin-bottom: 10px; }
                .success-message { color: #6c757d; margin-bottom: 30px; }
                .info { background: #d4edda; padding: 20px; border-radius: 4px; margin: 20px 0; border-left: 4px solid #28a745; }
                .close-btn { background: #6c757d; color: white; padding: 12px 24px; border: none; border-radius: 4px; cursor: pointer; margin-top: 15px; }
                .close-btn:hover { background: #545b62; }
                .status { font-size: 14px; color: #6c757d; margin-top: 15px; }
            </style>
        </head>
        <body>
            <div class="success-container">
                <div class="brand-header">🌮 OpenTaco</div>
                <h1 class="success-title">All Set!</h1>
                <p class="success-message">Your Terraform CLI has been successfully authenticated with OpenTaco.</p>
                
                <div class="info">
                    <p><strong>✅ Authentication Complete</strong></p>
                    <p>Your Terraform CLI is now ready to use with OpenTaco.</p>
                    <p>You can safely close this browser window.</p>
                </div>
                
                <button class="close-btn" onclick="completeCLI()" id="complete-btn">Complete CLI Authentication</button>
                <button class="close-btn" onclick="window.close()" id="close-btn" style="display:none">Close Window</button>
                <div class="status" id="status">Click "Complete CLI Authentication" to finish the process</div>
            </div>
            
            <script>
                function completeCLI() {
                    document.getElementById('status').textContent = 'Completing authentication...';
                    document.getElementById('complete-btn').style.display = 'none';
                    
                    // Redirect to CLI callback
                    window.location.href = '%s';
                }
                
                // Auto-complete after 3 seconds, or let user click
                setTimeout(function() {
                    if (document.getElementById('complete-btn').style.display !== 'none') {
                        completeCLI();
                    }
                }, 3000);
            </script>
        </body>
        </html>
    `, callbackURL)
    
    return c.HTML(http.StatusOK, html)
}

// Clean success page that never redirects
func (h *Handler) renderFinalSuccessPage(c echo.Context) error {
    html := `
        <html>
        <head>
            <title>OpenTaco - Authentication Complete</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; background: #f8f9fa; }
                .success-container { text-align: center; background: white; border: 1px solid #ddd; padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
                .brand-header { font-size: 32px; margin-bottom: 30px; font-weight: bold; }
                .success-title { color: #28a745; margin-bottom: 10px; }
                .success-message { color: #6c757d; margin-bottom: 30px; }
                .info { background: #d4edda; padding: 20px; border-radius: 4px; margin: 20px 0; border-left: 4px solid #28a745; }
                .close-btn { background: #6c757d; color: white; padding: 12px 24px; border: none; border-radius: 4px; cursor: pointer; margin-top: 15px; }
                .close-btn:hover { background: #545b62; }
            </style>
        </head>
        <body>
            <div class="success-container">
                <div class="brand-header">🌮 OpenTaco</div>
                <h1 class="success-title">All Set!</h1>
                <p class="success-message">Your Terraform CLI has been successfully authenticated with OpenTaco.</p>
                
                <div class="info">
                    <p><strong>✅ Authentication Complete</strong></p>
                    <p>Your Terraform CLI is now ready to use with OpenTaco.</p>
                    <p>You can safely close this browser window.</p>
                </div>
                
                <button class="close-btn" onclick="window.close()">Close Window</button>
            </div>
        </body>
        </html>
    `
    
    return c.HTML(http.StatusOK, html)
}

// DebugConfig handles GET /oauth/debug (for debugging OIDC config)
func (h *Handler) DebugConfig(c echo.Context) error {
    config, err := h.getServerAuthConfig()
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    
    return c.JSON(http.StatusOK, map[string]interface{}{
        "issuer": config.Issuer,
        "client_id": config.ClientID,
        "auth_url": config.AuthorizationEndpoint,
        "token_url": config.TokenEndpoint,
        "base_url": getBaseURL(c),
    })
}

// Helper functions

func (h *Handler) renderOAuthLoginPage(c echo.Context, sessionData map[string]string) error {
    baseURL := getBaseURL(c)
    
    html := fmt.Sprintf(`
        <html>
        <head>
            <title>OpenTaco - Terraform Login</title>
            <style>
                body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
                .login-container { text-align: center; border: 1px solid #ddd; padding: 40px; border-radius: 8px; }
                .brand-header { font-size: 32px; margin-bottom: 30px; font-weight: bold; }
                .btn { background: #007cba; color: white; padding: 12px 24px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 10px; }
                .btn:hover { background: #005a87; }
                .info { background: #f0f8ff; padding: 15px; border-radius: 4px; margin: 20px 0; }
            </style>
        </head>
        <body>
            <div class="login-container">
                <h2 class="brand-header">🌮 OpenTaco</h2>
                <h3>Terraform CLI Authentication</h3>
                
                <div class="info">
                    <p><strong>Terraform is requesting access to your OpenTaco account.</strong></p>
                    <p>Client: terraform-cli</p>
                    <p>Requested scopes: API access, S3 access</p>
                </div>
                
                <div>
                    <a href="%s/oauth/login-redirect?%s" class="btn">Sign in with OIDC</a>
                </div>
                
                <p><small>This will redirect you to your configured OIDC provider to authenticate.</small></p>
            </div>
        </body>
        </html>
    `, baseURL, encodeSessionData(sessionData))
    
    return c.HTML(http.StatusOK, html)
}

func getBaseURL(c echo.Context) string {
    // Prefer explicit public base URL if set (HA/LB safe)
    if pub := getenv("OPENTACO_PUBLIC_BASE_URL", ""); pub != "" {
        return pub
    }
    scheme := "http"
    if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
        scheme = "https"
    }
    if fwd := c.Request().Header.Get("X-Forwarded-Host"); fwd != "" {
        return fmt.Sprintf("%s://%s", scheme, fwd)
    }
    host := c.Request().Host
    return fmt.Sprintf("%s://%s", scheme, host)
}

func encodeSessionData(data map[string]string) string {
    params := url.Values{}
    for k, v := range data {
        params.Set(k, v)
    }
    return params.Encode()
}

func verifyPKCE(verifier, challenge string) bool {
    if verifier == "" || challenge == "" {
        return false
    }
    h := sha256.Sum256([]byte(verifier))
    computed := base64.RawURLEncoding.EncodeToString(h[:])
    return computed == challenge
}

// PKCE helper functions for server-side PKCE generation
func generateCodeVerifier() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base58.Encode(b)
}

func generateCodeChallenge(verifier string) string {
    h := sha256.Sum256([]byte(verifier))
    return base64.RawURLEncoding.EncodeToString(h[:])
}

// isValidTerraformRedirectURI ensures redirect_uri is a loopback HTTP URL on approved ports and path
func isValidTerraformRedirectURI(u string) bool {
    parsed, err := url.Parse(u)
    if err != nil { return false }
    if parsed.Scheme != "http" { return false }
    host := parsed.Hostname()
    if host != "127.0.0.1" && host != "localhost" && host != "::1" { return false }
    // Ports: default to Terraform discovery ports 10000 and 10010 inclusive
    portStr := parsed.Port()
    if portStr == "" { return false }
    port, err := strconv.Atoi(portStr)
    if err != nil { return false }
    if port < 10000 || port > 10010 { return false }
    // Allow common Terraform paths like /callback or /login
    if parsed.Path == "" { return false }
    // No strict suffix requirement to support /login used by some Terraform versions
    return true
}


// createAuthCode creates a JWT authorization code
func (h *Handler) createAuthCode(data *AuthCode) (string, error) {
    if h.signer == nil {
        return "", fmt.Errorf("JWT signer not available")
    }
    
    // Use the signer's OAuth code JWT method
    token, _, err := h.signer.MintOAuthCode(
        data.Subject,
        data.Email,
        data.ClientID,
        data.RedirectURI,
        data.CodeChallenge,
        data.Org,
        data.Groups,
    )
    if err != nil {
        return "", fmt.Errorf("failed to mint OAuth code: %w", err)
    }
    
    return token, nil
}

// verifyAuthCode verifies a JWT authorization code
func (h *Handler) verifyAuthCode(code string) (*AuthCode, error) {
    if h.signer == nil {
        return nil, fmt.Errorf("JWT signer not available")
    }
    
    // Verify the JWT and extract claims
    claims, err := h.signer.VerifyOAuthCode(code)
    if err != nil {
        return nil, fmt.Errorf("invalid or expired authorization code: %w", err)
    }
    
    // Convert JWT claims back to AuthCode struct
    authCode := &AuthCode{
        ClientID:      claims.ClientID,
        RedirectURI:   claims.RedirectURI,
        Subject:       claims.Subject,
        Email:         claims.Email,
        Groups:        claims.Groups,
        CodeChallenge: claims.CodeChallenge,
        Org:           claims.Org,
    }
    
    return authCode, nil
}

// encryptOAuthSession encrypts OAuth session data into a state parameter
func encryptOAuthSession(data *OAuthSession, key []byte) (string, error) {
    // Marshal the session data
    jsonData, err := json.Marshal(data)
    if err != nil {
        return "", err
    }
    
    // Use generic AES-GCM encryption
    return encryptAESGCM(jsonData, key)
}

// decryptOAuthSession decrypts OAuth session data from a state parameter
func decryptOAuthSession(encryptedState string, key []byte) (*OAuthSession, error) {
    // Use generic AES-GCM decryption
    jsonData, err := decryptAESGCM(encryptedState, key)
    if err != nil {
        return nil, fmt.Errorf("failed to decrypt OAuth session: %w", err)
    }
    
    // Unmarshal session data
    var data OAuthSession
    if err := json.Unmarshal(jsonData, &data); err != nil {
        return nil, fmt.Errorf("invalid session data: %w", err)
    }
    
    return &data, nil
}

// getEncryptionKey generates or retrieves the encryption key for OAuth state
func getEncryptionKey() []byte {
    key := getenv("OPENTACO_OAUTH_STATE_KEY", "default-key-change-in-production-32b")
    if key == "default-key-change-in-production-32b" {
        log.Printf("OAuth: WARNING - Using default state encryption key, change OPENTACO_OAUTH_STATE_KEY in production")
    } else {
        log.Printf("OAuth: Using custom state encryption key")
    }
    
    // Ensure key is 32 bytes for AES-256
    h := sha256.Sum256([]byte(key))
    return h[:]
}

func (h *Handler) getServerAuthConfig() (*serverAuthConfig, error) {
    issuer := getenv("OPENTACO_AUTH_ISSUER", "")
    clientID := getenv("OPENTACO_AUTH_CLIENT_ID", "")
    clientSecret := getenv("OPENTACO_AUTH_CLIENT_SECRET", "")
    authURL := getenv("OPENTACO_AUTH_AUTH_URL", "")
    tokenURL := getenv("OPENTACO_AUTH_TOKEN_URL", "")
    
    log.Printf("OIDC: Configuration loaded - Issuer: %s, Client ID: %s, Auth URL: %s, Token URL: %s", 
        issuer, clientID, authURL, tokenURL)
    
    // Provide defaults for WorkOS
    if issuer == "https://api.workos.com/user_management" {
        if authURL == "" { authURL = "https://api.workos.com/user_management/authorize" }
        if tokenURL == "" { tokenURL = "https://api.workos.com/user_management/token" }
    }
    
    return &serverAuthConfig{
        Issuer:                issuer,
        ClientID:              clientID,
        ClientSecret:          clientSecret,
        AuthorizationEndpoint: authURL,
        TokenEndpoint:         tokenURL,
    }, nil
}

type oidcTokenResponse struct {
    IDToken string `json:"id_token"`
}

func (h *Handler) exchangeOIDCCode(config *serverAuthConfig, code, redirectURI, codeVerifier string) (*oidcTokenResponse, error) {
    form := url.Values{}
    form.Set("grant_type", "authorization_code")
    form.Set("code", code)
    form.Set("client_id", config.ClientID)
    form.Set("redirect_uri", redirectURI)
    if config.ClientSecret != "" {
        form.Set("client_secret", config.ClientSecret)
    }
    // Include PKCE code verifier if provided (required when PKCE is used)
    if codeVerifier != "" {
        form.Set("code_verifier", codeVerifier)
    }
    
    req, err := http.NewRequest("POST", config.TokenEndpoint, strings.NewReader(form.Encode()))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        var b strings.Builder
        io.Copy(&b, resp.Body)
        return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, b.String())
    }
    
    var tokenResp oidcTokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }
    
    return &tokenResp, nil
}

// ensureUserHasOrg ensures a user exists and has an org, auto-creating both if needed
func (h *Handler) ensureUserHasOrg(ctx context.Context, subject, email string) (string, error) {
    if h.db == nil || h.orgRepo == nil {
        return "", fmt.Errorf("db or org repository not configured")
    }

    orgs, err := h.getUserOrganizations(ctx, subject)
    if err != nil {
        return "", fmt.Errorf("failed to get user orgs: %w", err)
    }

    if len(orgs) > 0 {
        return orgs[0], nil
    }

    // If user has no RBAC membership yet, reuse any org they previously created
    // This avoids creating multiple orgs before membership is assigned
    var existing struct{ ID string }
    if err := h.db.WithContext(ctx).
        Table("organizations").
        Select("id").
        Where("created_by = ?", subject).
        Order("created_at DESC").
        First(&existing).Error; err == nil && existing.ID != "" {
        return existing.ID, nil
    }

    orgName := fmt.Sprintf("user-%s", subject[:min(8, len(subject))])
    orgDisplayName := fmt.Sprintf("%s's Organization", email)

    org, err := h.orgRepo.Create(ctx, orgName, orgDisplayName, subject)
    if err != nil {
        return "", fmt.Errorf("failed to create org: %w", err)
    }

    return org.ID, nil
}

func (h *Handler) getUserOrganizations(ctx context.Context, subject string) ([]string, error) {
    var userID string
    if err := h.db.WithContext(ctx).Table("users").Where("subject = ?", subject).Pluck("id", &userID).Error; err != nil {
        return nil, err
    }
    if userID == "" {
        return []string{}, nil
    }

    var orgIDs []string
    if err := h.db.WithContext(ctx).Table("user_roles").Where("user_id = ?", userID).Pluck("DISTINCT org_id", &orgIDs).Error; err != nil {
        return nil, err
    }

    return orgIDs, nil
}
