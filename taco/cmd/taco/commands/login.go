package commands

import (
    "bufio"
    "context"
    "bytes"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "runtime"
    "strings"
    "strconv"
    "time"

    "github.com/diggerhq/digger/opentaco/pkg/sdk"
    "github.com/spf13/cobra"
    "github.com/mr-tron/base58"
)

var (
    idToken   string
    issuer    string
    clientID  string
    scopes    string
    cbPort    int
    authURL   string
    tokenURL  string
    forceLogin bool
)

var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Authenticate via OIDC (PKCE or --id-token)",
    RunE: func(cmd *cobra.Command, args []string) error {
        base := normalizedBase(serverURL)

        // If user supplied an ID token directly, exchange it.
        if idToken != "" {
            err := exchangeIDToken(base, idToken)
            if err != nil {
                return err
            }
            // After successful login, prompt for analytics email
            return promptForAnalyticsEmail(base)
        }

        if issuer == "" || clientID == "" {
            // Try to fetch from server
            if sc, err := fetchServerAuthConfig(base); err == nil {
                if issuer == "" { issuer = sc.Issuer }
                if clientID == "" { clientID = sc.ClientID }
                if authURL == "" && sc.AuthorizationEndpoint != "" { authURL = sc.AuthorizationEndpoint }
                if tokenURL == "" && sc.TokenEndpoint != "" { tokenURL = sc.TokenEndpoint }
                if cbPort == 8585 && len(sc.RedirectURIs) > 0 {
                    if u, err := url.Parse(sc.RedirectURIs[0]); err == nil && u.Port() != "" {
                        if p, err := strconv.Atoi(u.Port()); err == nil { cbPort = p }
                    }
                }
            }
        }
        if issuer == "" || clientID == "" { return fmt.Errorf("OpenTaco could not discover your issuer/client-id, are you sure the server is running?; set flags or configure server /v1/auth/config") }

        // Discover endpoints or use overrides
        var authEp, tokenEp string
        if authURL != "" && tokenURL != "" {
            authEp, tokenEp = authURL, tokenURL
        } else {
            wc, err := discoverOIDC(issuer)
            if err != nil { return fmt.Errorf("discovery failed: %w", err) }
            authEp, tokenEp = wc.AuthorizationEndpoint, wc.TokenEndpoint
        }

        // PKCE setup
        verifier := randomB58(32)
        challenge := codeChallengeS256(verifier)
        state := randomB58(24)
        redirectURI := fmt.Sprintf("http://localhost:%d/callback", cbPort)

        // Start local callback server (dedicated mux)
        codeCh := make(chan url.Values, 1)
        mux := http.NewServeMux()
        srv := &http.Server{Addr: fmt.Sprintf("localhost:%d", cbPort), Handler: mux}
        mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
            q := r.URL.Query()
            if q.Get("state") != state {
                http.Error(w, "state mismatch", http.StatusBadRequest)
                return
            }
            w.Header().Set("Content-Type", "text/plain")
            io.WriteString(w, "Login successful. You can close this window.")
            go func() {
                codeCh <- q
            }()
            go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
                defer cancel()
                _ = srv.Shutdown(ctx)
            }()
        })
        ln, err := net.Listen("tcp", srv.Addr)
        if err != nil { return fmt.Errorf("listen %s: %w", srv.Addr, err) }
        go func() { _ = srv.Serve(ln) }()

        // Build auth URL
        v := url.Values{}
        v.Set("response_type", "code")
        v.Set("client_id", clientID)
        v.Set("redirect_uri", redirectURI)
        v.Set("scope", scopes)
        v.Set("state", state)
        v.Set("code_challenge", challenge)
        v.Set("code_challenge_method", "S256")
        if forceLogin {
            v.Set("prompt", "login")
            v.Set("max_age", "0")
        }
        authPage := authEp + "?" + v.Encode()
        fmt.Println("Open the following URL to login:")
        fmt.Println(authPage)
        _ = openBrowser(authPage)

        // Wait for callback
        select {
        case q := <-codeCh:
            code := q.Get("code")
            if code == "" { return fmt.Errorf("missing code in callback") }
            // Exchange for tokens (expecting id_token)
            tok, err := exchangeCodeForTokens(tokenEp, clientID, redirectURI, code, verifier)
            if err != nil { return err }
            if tok.IDToken == "" { return fmt.Errorf("no id_token in token response") }
            err = exchangeIDToken(base, tok.IDToken)
            if err != nil {
                return err
            }
            // After successful login, prompt for analytics email
            return promptForAnalyticsEmail(base)
        case <-time.After(5 * time.Minute):
            _ = srv.Close()
            return fmt.Errorf("login timed out")
        }
    },
}

func init() {
    loginCmd.Flags().StringVar(&idToken, "id-token", "", "OIDC ID token to exchange (advanced)")
    loginCmd.Flags().StringVar(&issuer, "issuer", getEnvOrDefault("OPENTACO_AUTH_ISSUER", ""), "OIDC issuer URL")
    loginCmd.Flags().StringVar(&clientID, "client-id", getEnvOrDefault("OPENTACO_AUTH_CLIENT_ID", ""), "OIDC client ID")
    loginCmd.Flags().StringVar(&scopes, "scopes", "openid profile email offline_access", "OIDC scopes")
    loginCmd.Flags().IntVar(&cbPort, "callback-port", 8585, "Loopback callback port")
    loginCmd.Flags().StringVar(&authURL, "auth-url", "", "Override authorization endpoint URL")
    loginCmd.Flags().StringVar(&tokenURL, "token-url", "", "Override token endpoint URL")
    loginCmd.Flags().BoolVar(&forceLogin, "force-login", false, "Force re-auth even if SSO session exists (adds prompt=login)")
    rootCmd.AddCommand(loginCmd)
}

func exchangeIDToken(base, idTok string) error {
    body := map[string]string{"id_token": idTok}
    data, _ := json.Marshal(body)
    req, err := http.NewRequest("POST", base+"/v1/auth/exchange", bytes.NewReader(data))
    if err != nil { return err }
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return fmt.Errorf("exchange failed: HTTP %d", resp.StatusCode) }
    var tok struct{
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil { return err }
    if tok.AccessToken == "" {
        return fmt.Errorf("server returned empty access token")
    }
    cf, err := loadCreds()
    if err != nil { return err }
    cf.Profiles[base] = tokens{AccessToken: tok.AccessToken, RefreshToken: tok.RefreshToken}
    if err := saveCreds(cf); err != nil { return err }
    fmt.Println("Login successful; tokens saved.")
    return nil
}

type wellKnown struct {
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint         string `json:"token_endpoint"`
}

func discoverOIDC(issuer string) (*wellKnown, error) {
    // Ensure trailing slash stripped
    base := strings.TrimRight(issuer, "/")
    url := base + "/.well-known/openid-configuration"
    resp, err := http.Get(url)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return nil, fmt.Errorf("discovery http %d", resp.StatusCode) }
    var wc wellKnown
    if err := json.NewDecoder(resp.Body).Decode(&wc); err != nil { return nil, err }
    if wc.AuthorizationEndpoint == "" || wc.TokenEndpoint == "" { return nil, errors.New("incomplete discovery") }
    return &wc, nil
}

func codeChallengeS256(verifier string) string {
    h := sha256.Sum256([]byte(verifier))
    return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomB58(n int) string {
    b := make([]byte, n)
    rand.Read(b)
    return base58.Encode(b)
}

func exchangeCodeForTokens(tokenURL, clientID, redirectURI, code, verifier string) (*struct{ IDToken string `json:"id_token"`}, error) {
    form := url.Values{}
    form.Set("grant_type", "authorization_code")
    form.Set("code", code)
    form.Set("client_id", clientID)
    form.Set("redirect_uri", redirectURI)
    form.Set("code_verifier", verifier)
    req, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
    if err != nil { return nil, err }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return nil, fmt.Errorf("token http %d", resp.StatusCode) }
    var out struct{ IDToken string `json:"id_token"` }
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return nil, err }
    return &out, nil
}

func openBrowser(u string) error {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin":
        cmd = exec.Command("open", u)
    case "windows":
        cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
    default:
        cmd = exec.Command("xdg-open", u)
    }
    return cmd.Start()
}

// Server-side config shape
type serverAuthConfig struct {
    Issuer                string   `json:"issuer"`
    ClientID              string   `json:"client_id"`
    AuthorizationEndpoint string   `json:"authorization_endpoint"`
    TokenEndpoint         string   `json:"token_endpoint"`
    RedirectURIs          []string `json:"redirect_uris"`
}

func fetchServerAuthConfig(base string) (*serverAuthConfig, error) {
    resp, err := http.Get(base + "/v1/auth/config")
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return nil, fmt.Errorf("config http %d", resp.StatusCode) }
    var out serverAuthConfig
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return nil, err }
    return &out, nil
}

// promptForAnalyticsEmail prompts for analytics email after successful login
func promptForAnalyticsEmail(base string) error {
    client := sdk.NewClient(base)
    
    // Check if server is using S3 storage
    storageInfo, err := getStorageInfo(client)
    if err != nil {
        // If we can't determine storage type, skip email prompt
        return nil
    }

    if storageInfo.Type != "s3" {
        return nil // Not using S3, no need for email
    }

    // Check if user email is already set
    existingEmail, err := getUserEmailFromServer(client)
    if err != nil {
        // If we can't check the email, skip the prompt to avoid blocking login
        fmt.Printf("Warning: Could not check existing analytics email: %v\n", err)
        return nil
    }
    
    fmt.Printf("DEBUG: Retrieved email from server: '%s'\n", existingEmail)
    
    if existingEmail != "" {
        fmt.Printf("Analytics email already set: %s\n", existingEmail)
        return nil // Email already set
    }

    // Prompt for analytics email
    fmt.Println("\n📊 Analytics Email (Optional)")
    fmt.Println("You've successfully logged in! To help improve OpenTaco and provide better support,")
    fmt.Println("we'd like to collect an email address for analytics purposes.")
    fmt.Println("This is completely separate from your login credentials and is optional.")
    fmt.Println()

    reader := bufio.NewReader(os.Stdin)
    
    for {
        fmt.Print("Enter an email for analytics (or press Enter to skip): ")
        email, err := reader.ReadString('\n')
        if err != nil {
            return err
        }
        
        email = strings.TrimSpace(email)
        
        if email == "" {
            fmt.Println("Skipping analytics email collection.")
            return nil
        }
        
        // Basic email validation
        if isValidEmail(email) {
            // Send the email to the server
            if err := setUserEmailOnServer(client, email); err != nil {
                fmt.Printf("Warning: Failed to save analytics email: %v\n", err)
                return nil // Don't fail the login
            }
            
            fmt.Printf("✓ Analytics email saved: %s\n", email)
            return nil
        }
        
        fmt.Println("Please enter a valid email address or press Enter to skip.")
    }
}
