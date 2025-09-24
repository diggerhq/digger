package commands

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "text/tabwriter"
    "time"

    "github.com/diggerhq/digger/opentaco/internal/analytics"
    "github.com/diggerhq/digger/opentaco/pkg/sdk"
    "github.com/google/uuid"
    "github.com/spf13/cobra"
)

// unitCmd represents the unit command
var unitCmd = &cobra.Command{
    Use:   "unit",
    Short: "Manage OpenTaco units",
    Long:  `Manage OpenTaco units including create, list, delete, lock/unlock, and data operations.`,
}

func init() {
    // Add base command to root
    rootCmd.AddCommand(unitCmd)

    // Add subcommands
    unitCmd.AddCommand(unitCreateCmd)
    unitCmd.AddCommand(unitListCmd)
    unitCmd.AddCommand(unitInfoCmd)
    unitCmd.AddCommand(unitDeleteCmd)
    unitCmd.AddCommand(unitPullCmd)
    unitCmd.AddCommand(unitPushCmd)
    unitCmd.AddCommand(unitLockCmd)
    unitCmd.AddCommand(unitUnlockCmd)
    unitCmd.AddCommand(unitAcquireCmd)
    unitCmd.AddCommand(unitReleaseCmd)
    unitCmd.AddCommand(unitVersionsCmd)
    unitCmd.AddCommand(unitRestoreCmd)
    unitCmd.AddCommand(unitStatusCmd)
    
    // Tag management subcommands
    unitCmd.AddCommand(unitTagCmd)
}

var (
    unitCreateTags        string
    unitCreateOrg         string  
    unitCreateDescription string
)

var unitCreateCmd = &cobra.Command{
    Use:   "create <unit-id>",
    Short: "Create a new unit with optional tags and metadata",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        analytics.SendEssential("taco_unit_create_started")
        
        client := newAuthedClient()
        unitID := args[0]

        printVerbose("Creating unit: %s", unitID)

        // Parse tags if provided
        var tags []string
        if unitCreateTags != "" {
            tags = strings.Split(unitCreateTags, ",")
            for i, tag := range tags {
                tags[i] = strings.TrimSpace(tag)
            }
        }

        // Use enhanced creation with metadata if any metadata provided
        if len(tags) > 0 || unitCreateOrg != "" || unitCreateDescription != "" {
            // For now, we'll use the TFE workspace API to create units with metadata
            // TODO: Add direct SDK support for CreateWithMetadata
            fmt.Printf("Creating unit with tags %v, org: %s, description: %s\n", tags, unitCreateOrg, unitCreateDescription)
            resp, err := client.CreateUnit(context.Background(), unitID)
            if err != nil {
                analytics.SendEssential("taco_unit_create_failed")
                return fmt.Errorf("failed to create unit: %w", err)
            }
            fmt.Printf("Unit created: %s (Note: tag/metadata support via CLI coming soon - use TFE API for now)\n", resp.ID)
        } else {
            // Simple creation
            resp, err := client.CreateUnit(context.Background(), unitID)
            if err != nil {
                analytics.SendEssential("taco_unit_create_failed")
                return fmt.Errorf("failed to create unit: %w", err)
            }
            fmt.Printf("Unit created: %s\n", resp.ID)
        }

        analytics.SendEssential("taco_unit_create_completed")
        return nil
    },
}

func init() {
    unitCreateCmd.Flags().StringVar(&unitCreateTags, "tags", "", "Comma-separated list of tags (e.g., 'env:prod,app:web')")
    unitCreateCmd.Flags().StringVar(&unitCreateOrg, "org", "", "Organization name")
    unitCreateCmd.Flags().StringVar(&unitCreateDescription, "description", "", "Unit description")
}

var (
    unitStatusPrefix string
    unitStatusOutput string
)

var unitStatusCmd = &cobra.Command{
    Use:   "status [unit-id]",
    Short: "Show dependency status for a unit or prefix",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()

        var units []string
        pfx := strings.TrimSpace(unitStatusPrefix)
        if pfx == "/" { pfx = "" }

        if len(args) == 1 {
            units = []string{args[0]}
        } else {
            resp, err := client.ListUnits(context.Background(), pfx)
            if err != nil {
                return fmt.Errorf("failed to list units: %w", err)
            }
            for _, u := range resp.Units {
                units = append(units, u.ID)
            }
        }

        type row struct { Unit string; Status string; Pending int; First string }
        rows := make([]row, 0, len(units))
        results := make([]*sdk.UnitStatus, 0, len(units))
        for _, id := range units {
            st, err := client.GetUnitStatus(context.Background(), id)
            if err != nil {
                return fmt.Errorf("failed to get status for %s: %w", id, err)
            }
            results = append(results, st)
            first := ""
            for _, in := range st.Incoming {
                if in.Status == "pending" {
                    first = fmt.Sprintf("%s/%s", in.FromUnitID, in.FromOutput)
                    break
                }
            }
            rows = append(rows, row{Unit: st.UnitID, Status: st.Status, Pending: st.Summary.IncomingPending, First: first})
        }

        if unitStatusOutput == "json" {
            b, _ := json.MarshalIndent(results, "", "  ")
            fmt.Println(string(b))
            return nil
        }

        w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "UNIT\tSTATUS\tPENDING\tFIRST OFFENDER")
        for _, r := range rows {
            fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.Unit, humanStatusColored(r.Status), r.Pending, r.First)
        }
        w.Flush()
        return nil
    },
}

func init() {
    unitStatusCmd.Flags().StringVar(&unitStatusPrefix, "prefix", "", "Prefix to filter units")
    unitStatusCmd.Flags().StringVarP(&unitStatusOutput, "output", "o", "table", "Output format: table|json")
}

var (
    unitListTags string
    unitListOrg  string
)

var unitListCmd = &cobra.Command{
    Use:     "ls [prefix]",
    Short:   "List units with optional tag filtering",
    Aliases: []string{"list"},
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        prefix := ""
        if len(args) > 0 { prefix = args[0] }
        
        if unitListTags != "" || unitListOrg != "" {
            printVerbose("Listing units with tags: %s, org: %s", unitListTags, unitListOrg)
            fmt.Printf("Filtering by tags: %s, org: %s\n", unitListTags, unitListOrg)
            fmt.Println("Note: Tag filtering via CLI coming soon - use TFE API for now")
            // TODO: Implement tag-based filtering
        } else {
            printVerbose("Listing units with prefix: %s", prefix)
        }

        resp, err := client.ListUnits(context.Background(), prefix)
        if err != nil {
            return fmt.Errorf("failed to list units: %w", err)
        }

        if len(resp.Units) == 0 {
            fmt.Println("No units found")
            return nil
        }

        // Filter by RBAC if enabled
        filtered := resp.Units
        if rbacEnabled {
            filtered, err = filterUnitsByRBAC(context.Background(), client, resp.Units)
            if err != nil {
                printVerbose("Warning: failed to filter units by RBAC: %v", err)
            }
        }

        if len(filtered) == 0 {
            fmt.Println("No units found (or no read access to any units)")
            return nil
        }

        w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "ID\tSIZE\tUPDATED\tLOCKED\tTAGS")
        for _, u := range filtered {
            locked := ""
            if u.Locked { locked = "yes" }
            // TODO: Show actual tags from unit metadata
            tags := "n/a"
            fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", u.ID, u.Size, u.Updated.Format("2006-01-02 15:04:05"), locked, tags)
        }
        w.Flush()
        fmt.Printf("\nTotal: %d units (showing %d with read access)\n", resp.Count, len(filtered))
        return nil
    },
}

func init() {
    unitListCmd.Flags().StringVar(&unitListTags, "tags", "", "Filter by comma-separated tags")
    unitListCmd.Flags().StringVar(&unitListOrg, "org", "", "Filter by organization")
}

var unitInfoCmd = &cobra.Command{
    Use:     "info <unit-id>",
    Short:   "Show unit metadata information",
    Aliases: []string{"show", "describe"},
    Args:    cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        unitID := args[0]
        printVerbose("Getting unit metadata: %s", unitID)
        unit, err := client.GetUnit(context.Background(), unitID)
        if err != nil { return fmt.Errorf("failed to get unit info: %w", err) }
        data, _ := json.MarshalIndent(unit, "", "  ")
        fmt.Println(string(data))
        return nil
    },
}

var unitDeleteCmd = &cobra.Command{
    Use:     "rm <unit-id>",
    Short:   "Delete a unit",
    Aliases: []string{"delete", "remove"},
    Args:    cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        unitID := args[0]
        printVerbose("Deleting unit: %s", unitID)
        if err := client.DeleteUnit(context.Background(), unitID); err != nil {
            return fmt.Errorf("failed to delete unit: %w", err)
        }
        fmt.Printf("Unit deleted: %s\n", unitID)
        return nil
    },
}

var unitPullCmd = &cobra.Command{
    Use:   "pull <unit-id> [output-file]",
    Short: "Download unit data",
    Args:  cobra.RangeArgs(1, 2),
    RunE: func(cmd *cobra.Command, args []string) error {
        analytics.SendEssential("taco_unit_pull_started")
        
        client := newAuthedClient()
        unitID := args[0]
        printVerbose("Downloading unit: %s", unitID)
        data, err := client.DownloadUnit(context.Background(), unitID)
        if err != nil { 
            analytics.SendEssential("taco_unit_pull_failed")
            return fmt.Errorf("failed to download unit: %w", err) 
        }
        if len(args) > 1 {
            outputFile := args[1]
            if err := os.WriteFile(outputFile, data, 0o644); err != nil {
                analytics.SendEssential("taco_unit_pull_failed")
                return fmt.Errorf("failed to write file: %w", err)
            }
            fmt.Printf("Unit downloaded to: %s\n", outputFile)
        } else {
            fmt.Print(string(data))
        }
        analytics.SendEssential("taco_unit_pull_completed")
        return nil
    },
}

var unitPushCmd = &cobra.Command{
    Use:   "push <unit-id> <input-file>",
    Short: "Upload unit data",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        analytics.SendEssential("taco_unit_push_started")
        
        client := newAuthedClient()
        unitID := args[0]
        inputFile := args[1]
        printVerbose("Uploading unit: %s from %s", unitID, inputFile)
        data, err := os.ReadFile(inputFile)
        if err != nil { 
            analytics.SendEssential("taco_unit_push_failed")
            return fmt.Errorf("failed to read file: %w", err) 
        }
        lockID := getLockID(unitID)
        if err := client.UploadUnit(context.Background(), unitID, data, lockID); err != nil {
            analytics.SendEssential("taco_unit_push_failed")
            return fmt.Errorf("failed to upload unit: %w", err)
        }
        analytics.SendEssential("taco_unit_push_completed")
        fmt.Printf("Unit uploaded: %s\n", unitID)
        return nil
    },
}

var unitLockCmd = &cobra.Command{
    Use:   "lock <unit-id>",
    Short: "Lock a unit",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        unitID := args[0]
        printVerbose("Locking unit: %s", unitID)
        lockInfo := &sdk.LockInfo{ID: uuid.New().String(), Who: fmt.Sprintf("taco@%s", getHostname()), Version: "1.0.0", Created: time.Now()}
        result, err := client.LockUnit(context.Background(), unitID, lockInfo)
        if err != nil { return fmt.Errorf("failed to lock unit: %w", err) }
        saveLockID(unitID, result.ID)
        fmt.Printf("Unit locked: %s (lock ID: %s)\n", unitID, result.ID)
        return nil
    },
}

var unitUnlockCmd = &cobra.Command{
    Use:   "unlock <unit-id> [lock-id]",
    Short: "Unlock a unit",
    Args:  cobra.RangeArgs(1, 2),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        unitID := args[0]
        lockID := ""
        if len(args) > 1 { lockID = args[1] } else { lockID = getLockID(unitID); if lockID == "" { return fmt.Errorf("no lock ID provided and none found for %s", unitID) } }
        printVerbose("Unlocking unit: %s with lock ID: %s", unitID, lockID)
        if err := client.UnlockUnit(context.Background(), unitID, lockID); err != nil { return fmt.Errorf("failed to unlock unit: %w", err) }
        removeLockID(unitID)
        fmt.Printf("Unit unlocked: %s\n", unitID)
        return nil
    },
}

var unitAcquireCmd = &cobra.Command{
    Use:   "acquire <unit-id> [output-file]",
    Short: "Acquire unit (pull + lock)",
    Args:  cobra.RangeArgs(1, 2),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := sdk.NewClient(serverURL)
        unitID := args[0]
        printVerbose("Acquiring unit: %s", unitID)
        lockInfo := &sdk.LockInfo{ID: uuid.New().String(), Who: fmt.Sprintf("taco@%s", getHostname()), Version: "1.0.0", Created: time.Now()}
        result, err := client.LockUnit(context.Background(), unitID, lockInfo)
        if err != nil { return fmt.Errorf("failed to lock unit: %w", err) }
        saveLockID(unitID, result.ID)
        data, err := client.DownloadUnit(context.Background(), unitID)
        if err != nil {
            client.UnlockUnit(context.Background(), unitID, result.ID)
            removeLockID(unitID)
            return fmt.Errorf("failed to download unit: %w", err)
        }
        if len(args) > 1 {
            outputFile := args[1]
            if err := os.WriteFile(outputFile, data, 0o644); err != nil { return fmt.Errorf("failed to write file: %w", err) }
            fmt.Printf("Unit acquired and saved to: %s (lock ID: %s)\n", outputFile, result.ID)
        } else {
            fmt.Print(string(data))
            fmt.Fprintf(os.Stderr, "\n[Unit acquired with lock ID: %s]\n", result.ID)
        }
        return nil
    },
}

var unitReleaseCmd = &cobra.Command{
    Use:   "release <unit-id> <input-file>",
    Short: "Release unit (push + unlock)",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := sdk.NewClient(serverURL)
        unitID := args[0]
        inputFile := args[1]
        printVerbose("Releasing unit: %s", unitID)
        lockID := getLockID(unitID)
        if lockID == "" { return fmt.Errorf("no lock ID found for unit %s - was it acquired?", unitID) }
        data, err := os.ReadFile(inputFile)
        if err != nil { return fmt.Errorf("failed to read file: %w", err) }
        if err := client.UploadUnit(context.Background(), unitID, data, lockID); err != nil { return fmt.Errorf("failed to upload unit: %w", err) }
        if err := client.UnlockUnit(context.Background(), unitID, lockID); err != nil { return fmt.Errorf("failed to unlock unit: %w", err) }
        removeLockID(unitID)
        fmt.Printf("Unit released: %s\n", unitID)
        return nil
    },
}

var unitVersionsCmd = &cobra.Command{
    Use:   "versions <unit-id>",
    Short: "List all versions of a unit",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        unitID := args[0]
        printVerbose("Listing versions for unit: %s", unitID)
        versions, err := client.ListUnitVersions(context.Background(), unitID)
        if err != nil { return fmt.Errorf("failed to list versions: %w", err) }
        if len(versions) == 0 { fmt.Println("No versions found"); return nil }
        w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "VERSION\tCREATED\tSIZE\tHASH")
        for i, v := range versions {
            fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", len(versions)-i, v.Timestamp.Format("2006-01-02 15:04:05"), v.Size, v.Hash)
        }
        w.Flush()
        fmt.Printf("\nTotal: %d versions\n", len(versions))
        return nil
    },
}

var unitRestoreCmd = &cobra.Command{
    Use:   "restore <unit-id> <version-number>",
    Short: "Restore unit to a previous version",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        client := newAuthedClient()
        unitID := args[0]
        versionNumStr := args[1]
        versionNum, err := strconv.Atoi(versionNumStr)
        if err != nil { return fmt.Errorf("invalid version number: %s", versionNumStr) }
        printVerbose("Restoring unit %s to version %d", unitID, versionNum)
        versions, err := client.ListUnitVersions(context.Background(), unitID)
        if err != nil { return fmt.Errorf("failed to list versions: %w", err) }
        if versionNum < 1 || versionNum > len(versions) {
            return fmt.Errorf("version %d not found (available: 1-%d)", versionNum, len(versions))
        }
        target := versions[len(versions)-versionNum]
        lockID := getLockID(unitID)
        if err := client.RestoreUnitVersion(context.Background(), unitID, target.Timestamp, lockID); err != nil { return fmt.Errorf("failed to restore version: %w", err) }
        fmt.Printf("Unit %s restored to version %d (hash: %s, created: %s)\n", unitID, versionNum, target.Hash, target.Timestamp.Format("2006-01-02 15:04:05"))
        return nil
    },
}

// Principal represents a user principal for RBAC checks  
type Principal struct {
    Subject string
    Email   string
    Roles   []string
    Groups  []string
}

// RBAC helpers adjusted for units
func filterUnitsByRBAC(ctx context.Context, client *sdk.Client, units []*sdk.UnitMetadata) ([]*sdk.UnitMetadata, error) {
    // If RBAC is not enabled, allow all
    enabled, err := isRBACEnabled(ctx, client)
    if err != nil || !enabled { return units, nil }
    // Filter units based on access
    var filtered []*sdk.UnitMetadata
    for _, u := range units {
        canRead, err := hasAccess(ctx, client, "unit.read", u.ID)
        if err != nil {
            // Skip this unit if permission check fails, don't fail entire operation
            continue
        }
        if canRead { filtered = append(filtered, u) }
    }
    return filtered, nil
}

func isRBACEnabled(ctx context.Context, client *sdk.Client) (bool, error) {
    resp, err := client.Get(ctx, "/v1/rbac/me")
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return false, nil
    }
    var status RBACStatus
    if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
        return false, err
    }
    return status.Enabled, nil
}

func hasAccess(ctx context.Context, client *sdk.Client, action, resource string) (bool, error) {
    payload := map[string]string{"action": action, "resource": resource}
    resp, err := client.PostJSON(ctx, "/v1/rbac/test", payload)
    if err != nil { return false, err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return false, nil }
    var result struct{ Allowed bool `json:"allowed"` }
    body, err := io.ReadAll(resp.Body)
    if err != nil { return false, err }
    if err := json.Unmarshal(body, &result); err != nil { return false, err }
    return result.Allowed, nil
}

// AccessPolicy and matchesRule kept for local policy simulations
type AccessPolicy struct {
    Effect    string
    Actions   []string
    Resources []string
}

func matchesRule(rule AccessPolicy, action, resource string) bool {
    actionMatch := false
    for _, ra := range rule.Actions { if ra == action || ra == "*" { actionMatch = true; break } }
    if !actionMatch { return false }
    for _, rr := range rule.Resources {
        if rr == resource || rr == "*" { return true }
        if strings.Contains(rr, "*") { pattern := strings.ReplaceAll(rr, "*", ".*"); if matched, _ := regexp.MatchString("^"+pattern+"$", resource); matched { return true } }
    }
    return false
}

// Lock file helpers are shared
func getLockID(unitID string) string {
    lockFile := filepath.Join(os.TempDir(), "opentaco-locks", strings.ReplaceAll(unitID, "/", "__")+".lock")
    data, err := os.ReadFile(lockFile)
    if err != nil { return "" }
    return strings.TrimSpace(string(data))
}

func saveLockID(unitID, lockID string) {
    dir := filepath.Join(os.TempDir(), "opentaco-locks")
    _ = os.MkdirAll(dir, 0o755)
    lockFile := filepath.Join(dir, strings.ReplaceAll(unitID, "/", "__")+".lock")
    _ = os.WriteFile(lockFile, []byte(lockID), 0o600)
}

func removeLockID(unitID string) {
    lockFile := filepath.Join(os.TempDir(), "opentaco-locks", strings.ReplaceAll(unitID, "/", "__")+".lock")
    _ = os.Remove(lockFile)
}

//
// Tag Management Commands
//

// unitTagCmd is the parent command for tag operations
var unitTagCmd = &cobra.Command{
    Use:   "tag",
    Short: "Manage unit tags",
    Long:  `Manage tags for units including add, remove, and list operations.`,
}

func init() {
    // Add tag subcommands
    unitTagCmd.AddCommand(unitTagAddCmd)
    unitTagCmd.AddCommand(unitTagRemoveCmd)
    unitTagCmd.AddCommand(unitTagListCmd)
    unitTagCmd.AddCommand(unitTagShowCmd)
    
    // Add global tag management commands to root
    rootCmd.AddCommand(tagCmd)
}

var unitTagAddCmd = &cobra.Command{
    Use:   "add <unit-id> <tag-name>",
    Short: "Add a tag to a unit",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Printf("Adding tag '%s' to unit '%s'\n", args[1], args[0])
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

var unitTagRemoveCmd = &cobra.Command{
    Use:   "remove <unit-id> <tag-name>",
    Short: "Remove a tag from a unit",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Printf("Removing tag '%s' from unit '%s'\n", args[1], args[0])
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

var unitTagListCmd = &cobra.Command{
    Use:   "list <unit-id>",
    Short: "List all tags for a unit",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Printf("Tags for unit '%s':\n", args[0])
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

var unitTagShowCmd = &cobra.Command{
    Use:   "show <tag-name>",
    Short: "Show all units with a specific tag",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Printf("Units with tag '%s':\n", args[0])
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

// Global tag management commands
var tagCmd = &cobra.Command{
    Use:   "tag",
    Short: "Global tag management",
    Long:  `Manage tags globally including create, list, delete operations.`,
}

func init() {
    // Add global tag subcommands
    tagCmd.AddCommand(tagCreateCmd)
    tagCmd.AddCommand(tagListCmd)
    tagCmd.AddCommand(tagDeleteCmd)
    tagCmd.AddCommand(tagDescribeCmd)
}

var tagCreateCmd = &cobra.Command{
    Use:   "create <tag-name>",
    Short: "Create a new tag",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        description, _ := cmd.Flags().GetString("description")
        
        // TODO: Implement via API call
        fmt.Printf("Creating tag '%s' with description: %s\n", args[0], description)
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

func init() {
    tagCreateCmd.Flags().String("description", "", "Tag description")
}

var tagListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all available tags",
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Println("Available tags:")
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

var tagDeleteCmd = &cobra.Command{
    Use:   "delete <tag-name>",
    Short: "Delete a tag (removes from all units)",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Printf("Deleting tag '%s' from all units\n", args[0])
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

var tagDescribeCmd = &cobra.Command{
    Use:   "describe <tag-name>",
    Short: "Show tag details and usage",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: Implement via API call
        fmt.Printf("Details for tag '%s':\n", args[0])
        fmt.Println("Note: CLI tag operations coming soon - use TFE API for now")
        return nil
    },
}

