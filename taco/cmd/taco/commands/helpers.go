package commands

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/diggerhq/digger/opentaco/pkg/sdk"
)

// Global toggle for client-side RBAC filtering (default on)
var rbacEnabled = false  // Temporarily disabled - server should do RBAC filtering

// RBACStatus mirrors minimal server response used by CLI checks
type RBACStatus struct { Enabled bool `json:"rbac_enabled"` }

// humanStatusColored maps internal status to friendly text + color for terminal output
func humanStatusColored(status string) string {
    switch strings.ToLower(status) {
    case "green":
        return ansiColor("up to date", 32) // green
    case "red":
        return ansiColor("needs re-apply", 31) // red
    case "yellow":
        return ansiColor("might need re-apply", 33) // yellow
    default:
        return status
    }
}

func ansiColor(text string, code int) string { return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, text) }

func getHostname() string {
    if h, err := os.Hostname(); err == nil && h != "" { return h }
    return "unknown"
}

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
    Features map[string]bool `json:"features"`
}

// CheckCapabilitiesAndPromptEmail checks server capabilities and prompts for email if analytics are enabled
func CheckCapabilitiesAndPromptEmail(client *sdk.Client) error {
    // Check server capabilities
    caps, err := getCapabilities(client)
    if err != nil {
        // If we can't determine capabilities, skip email prompt
        return nil
    }

    // Only prompt for email if the server needs it (has persistent storage)
    // We infer this from any capability being enabled (RBAC or query backend)
    needsEmail := caps.Features["rbac"] || caps.Features["query"]
    if !needsEmail {
        return nil
    }

    // Check if user email is already set by trying to get it from server
    existingEmail, err := getUserEmailFromServer(client)
    if err != nil {
        // If we can't check the email, skip the prompt to avoid blocking
        return nil
    }
    
    if existingEmail != "" {
        return nil // Email already set
    }

    // Prompt for email
    email, err := promptForEmail()
    if err != nil {
        return fmt.Errorf("failed to get email: %w", err)
    }

    if email == "" {
        return nil // User skipped
    }

    // Send the email to the server
    if err := setUserEmailOnServer(client, email); err != nil {
        return fmt.Errorf("failed to set user email: %w", err)
    }

    fmt.Printf("✓ User email set: %s\n", email)
    return nil
}

// getCapabilities queries the server for its capabilities
func getCapabilities(client *sdk.Client) (*ServerCapabilities, error) {
    resp, err := client.Get(context.Background(), "/v1/capabilities")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var caps ServerCapabilities
    if err := json.Unmarshal(body, &caps); err != nil {
        return nil, err
    }

    return &caps, nil
}

// promptForEmail prompts the user for their email address
func promptForEmail() (string, error) {
    fmt.Println("\n📊 OpenTaco Analytics")
    fmt.Println("You're connecting to an OpenTaco server with persistent storage.")
    fmt.Println("To help improve OpenTaco and provide better support, we'd like to collect your email address.")
    fmt.Println("This is completely optional and separate from authentication.")
    fmt.Println("Your email will be stored securely with your system ID for analytics purposes only.")
    fmt.Println()

    reader := bufio.NewReader(os.Stdin)
    
    for {
        fmt.Print("Enter your email address (or press Enter to skip): ")
        email, err := reader.ReadString('\n')
        if err != nil {
            return "", err
        }
        
        email = strings.TrimSpace(email)
        
        if email == "" {
            fmt.Println("Skipping email collection. You can set this later if needed.")
            return "", nil
        }
        
        // Basic email validation
        if isValidEmail(email) {
            return email, nil
        }
        
        fmt.Println("Please enter a valid email address or press Enter to skip.")
    }
}

// isValidEmail performs basic email validation
func isValidEmail(email string) bool {
    return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// getUserEmailFromServer gets the current user email from the server
func getUserEmailFromServer(client *sdk.Client) (string, error) {
    resp, err := client.Get(context.Background(), "/v1/system-id/user-email")
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return "", fmt.Errorf("server returned status %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }

    return strings.TrimSpace(string(body)), nil
}

// setUserEmailOnServer sets the user email on the server
func setUserEmailOnServer(client *sdk.Client, email string) error {
    data := map[string]string{"email": email}
    resp, err := client.PostJSON(context.Background(), "/v1/system-id/user-email", data)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("server returned status %d", resp.StatusCode)
    }

    return nil
}

