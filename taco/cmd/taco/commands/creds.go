package commands

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"

    "github.com/spf13/cobra"
)

var credsCmd = &cobra.Command{
    Use:   "creds",
    Short: "Issue short-lived S3 credentials (JSON)",
    Args:  cobra.NoArgs,
}

var credsJSON bool

func init() {
    credsCmd.Flags().BoolVar(&credsJSON, "json", false, "Output AWS Process Credentials JSON")
    credsCmd.RunE = runCreds
    rootCmd.AddCommand(credsCmd)
}

func runCreds(cmd *cobra.Command, args []string) error {
    if !credsJSON {
        fmt.Fprintln(os.Stderr, "Use --json for AWS credential_process output")
        return fmt.Errorf("missing --json flag")
    }
    base := normalizedBase(serverURL)
    cf, err := loadCreds()
    if err != nil { return err }
    tok, ok := cf.Profiles[base]
    if !ok || tok.AccessToken == "" {
        return fmt.Errorf("no credentials found for %s; run 'taco login' first", base)
    }
    // For now POST empty body; Authorization is what matters
    req, err := http.NewRequest("POST", base+"/v1/auth/issue-s3-creds", bytes.NewReader([]byte("{}")))
    if err != nil { return err }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return fmt.Errorf("issue-s3-creds failed: HTTP %d", resp.StatusCode)
    }
    // Pass JSON through to stdout
    var payload map[string]any
    if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil { return err }
    enc := json.NewEncoder(os.Stdout)
    enc.SetEscapeHTML(false)
    return enc.Encode(payload)
}
