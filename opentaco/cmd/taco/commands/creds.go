package commands

import (
    "encoding/json"
    "fmt"
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
    // Stubbed output to keep shape; real values will be provided by the service later.
    payload := map[string]any{
        "Version":         1,
        "AccessKeyId":     "OTC.k1.DUMMY",
        "SecretAccessKey": "DUMMY",
        "SessionToken":    "DUMMY",
        "Expiration":      "2099-01-01T00:00:00Z",
    }
    enc := json.NewEncoder(os.Stdout)
    enc.SetEscapeHTML(false)
    return enc.Encode(payload)
}

