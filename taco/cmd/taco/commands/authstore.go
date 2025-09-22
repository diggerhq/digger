package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/pkg/sdk"
)

type credsFile struct {
    Profiles map[string]tokens `json:"profiles"`
}

type tokens struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

// Terraform credentials file format (.terraform.d/credentials.tfrc.json)
type terraformCredsFile struct {
    Credentials map[string]terraformHostCreds `json:"credentials"`
}

type terraformHostCreds struct {
    Token string `json:"token"`
}

func configDir() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil { return "", err }
    dir := filepath.Join(home, ".config", "opentaco")
    if err := os.MkdirAll(dir, 0o755); err != nil { return "", err }
    return dir, nil
}

func credsPath() (string, error) {
    dir, err := configDir()
    if err != nil { return "", err }
    return filepath.Join(dir, "credentials.json"), nil
}

func loadCreds() (*credsFile, error) {
    path, err := credsPath()
    if err != nil { return nil, err }
    b, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return &credsFile{Profiles: map[string]tokens{}}, nil
    }
    if err != nil { return nil, err }
    var f credsFile
    if err := json.Unmarshal(b, &f); err != nil { return nil, err }
    if f.Profiles == nil { f.Profiles = map[string]tokens{} }
    return &f, nil
}

func saveCreds(cf *credsFile) error {
    path, err := credsPath()
    if err != nil { return err }
    b, err := json.MarshalIndent(cf, "", "  ")
    if err != nil { return err }
    return os.WriteFile(path, b, 0o600)
}

func normalizedBase(base string) string {
    return strings.TrimRight(base, "/")
}

// terraformCredsPath returns the path to Terraform's credential file
func terraformCredsPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil { return "", err }
    return filepath.Join(home, ".terraform.d", "credentials.tfrc.json"), nil
}

// loadTerraformCreds loads credentials from Terraform's credential file
func loadTerraformCreds() (*terraformCredsFile, error) {
    path, err := terraformCredsPath()
    if err != nil { return nil, err }
    
    b, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return &terraformCredsFile{Credentials: map[string]terraformHostCreds{}}, nil
    }
    if err != nil { return nil, err }
    
    var f terraformCredsFile
    if err := json.Unmarshal(b, &f); err != nil { return nil, err }
    if f.Credentials == nil { f.Credentials = map[string]terraformHostCreds{} }
    return &f, nil
}

// extractHostFromURL extracts hostname:port from a URL for Terraform credential lookup
func extractHostFromURL(serverURL string) (string, error) {
    u, err := url.Parse(serverURL)
    if err != nil { return "", err }
    return u.Host, nil
}

// getTerraformCandidateHosts returns possible host formats to try for Terraform credential lookup
func getTerraformCandidateHosts(serverURL string) []string {
    var candidates []string
    
    // Parse the URL
    u, err := url.Parse(serverURL)
    if err == nil {
        // Add host:port format (most common for terraform login)
        candidates = append(candidates, u.Host)
        
        // Add hostname only (without port) 
        if strings.Contains(u.Host, ":") {
            hostname := strings.Split(u.Host, ":")[0]
            candidates = append(candidates, hostname)
        }
    }
    
    // Add the full URL as fallback
    candidates = append(candidates, serverURL)
    
    // Add normalized URL without trailing slash
    candidates = append(candidates, strings.TrimRight(serverURL, "/"))
    
    return candidates
}

// Global variables to store both credentials
var (
    cachedOpenTacoToken string
    cachedTerraformToken string
    cachedBaseURL string
)

func newAuthedClient() *sdk.Client {
    base := normalizedBase(serverURL)
    
    // Create client with resilient HTTP transport
    resilientHTTPClient := createResilientHTTPClient()
    c := sdk.NewClientWithHTTPClient(base, resilientHTTPClient)
    
    // Reset cached tokens to avoid stale values after logout
    cachedOpenTacoToken = ""
    cachedTerraformToken = ""
    cachedBaseURL = base
    
    // Load OpenTaco credentials
    cf, err := loadCreds()
    if err == nil {
        if t, ok := cf.Profiles[base]; ok && t.AccessToken != "" {
            cachedOpenTacoToken = t.AccessToken
        } else if len(cf.Profiles) == 1 {
            for _, t := range cf.Profiles {
                if t.AccessToken != "" {
                    cachedOpenTacoToken = t.AccessToken
                    break
                }
            }
        }
    }
    
    // Load Terraform credentials
    tcf, err := loadTerraformCreds()
    if err == nil {
        candidateHosts := getTerraformCandidateHosts(base)
        for _, host := range candidateHosts {
            if creds, ok := tcf.Credentials[host]; ok && creds.Token != "" {
                cachedTerraformToken = creds.Token
                break
            }
        }
    }
    
    // Set primary credential (prefer Terraform if both exist)
    if cachedTerraformToken != "" {
        fmt.Printf("[CREDS DEBUG] Primary: terraform token\n")
        c.SetBearerToken(cachedTerraformToken)
    } else if cachedOpenTacoToken != "" {
        fmt.Printf("[CREDS DEBUG] Primary: opentaco token\n") 
        c.SetBearerToken(cachedOpenTacoToken)
    }
    
    return c
}

// Enhanced HTTP client that wraps the standard one with retry logic
func createResilientHTTPClient() *http.Client {
    originalTransport := http.DefaultTransport.(*http.Transport).Clone()
    
    return &http.Client{
        Transport: &retryTransport{
            base: originalTransport,
        },
        Timeout: 30 * time.Second,
    }
}

// retryTransport wraps the HTTP transport to retry with alternate credentials on 401
type retryTransport struct {
    base http.RoundTripper
}

func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Clone the request for potential retry
    originalReq := req.Clone(req.Context())
    
    // Make the first request
    resp, err := rt.base.RoundTrip(req)
    if err != nil {
        return resp, err
    }
    
    // If we get 401 and have alternate credentials, retry
    if resp.StatusCode == 401 && rt.hasAlternateToken(req) {
        resp.Body.Close() // Close first response
        
        fmt.Printf("[AUTH RETRY] 401 with primary credential, trying alternate\n")
        
        // Switch to alternate token in the cloned request
        rt.switchToAlternateToken(originalReq)
        
        // Retry the request
        return rt.base.RoundTrip(originalReq)
    }
    
    return resp, nil
}

func (rt *retryTransport) hasAlternateToken(req *http.Request) bool {
    authHeader := req.Header.Get("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return false
    }
    
    currentToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
    return (currentToken == cachedOpenTacoToken && cachedTerraformToken != "") ||
           (currentToken == cachedTerraformToken && cachedOpenTacoToken != "")
}

func (rt *retryTransport) switchToAlternateToken(req *http.Request) {
    authHeader := req.Header.Get("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return
    }
    
    currentToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
    var alternateToken string
    
    if currentToken == cachedOpenTacoToken && cachedTerraformToken != "" {
        alternateToken = cachedTerraformToken
        fmt.Printf("[AUTH RETRY] Switching from OpenTaco to Terraform credentials\n")
    } else if currentToken == cachedTerraformToken && cachedOpenTacoToken != "" {
        alternateToken = cachedOpenTacoToken
        fmt.Printf("[AUTH RETRY] Switching from Terraform to OpenTaco credentials\n")
    }
    
    if alternateToken != "" {
        req.Header.Set("Authorization", "Bearer "+alternateToken)
    }
}
