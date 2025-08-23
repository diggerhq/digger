package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/diggerhq/digger/opentaco/pkg/sdk"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// stateCmd represents the state command
var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage Terraform states",
	Long:  `Manage Terraform states including create, list, delete, lock/unlock, and data operations.`,
}

func init() {
	// Add subcommands
	stateCmd.AddCommand(createCmd)
	stateCmd.AddCommand(listCmd)
	stateCmd.AddCommand(infoCmd)
	stateCmd.AddCommand(deleteCmd)
	stateCmd.AddCommand(pullCmd)
	stateCmd.AddCommand(pushCmd)
	stateCmd.AddCommand(lockCmd)
	stateCmd.AddCommand(unlockCmd)
	stateCmd.AddCommand(acquireCmd)
	stateCmd.AddCommand(releaseCmd)
}

var createCmd = &cobra.Command{
	Use:   "create <state-id>",
	Short: "Create a new state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]

		printVerbose("Creating state: %s", stateID)
		
		resp, err := client.CreateState(context.Background(), stateID)
		if err != nil {
			return fmt.Errorf("failed to create state: %w", err)
		}

		fmt.Printf("State created: %s\n", resp.ID)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "ls [prefix]",
	Short: "List states",
	Aliases: []string{"list"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		
		prefix := ""
		if len(args) > 0 {
			prefix = args[0]
		}

		printVerbose("Listing states with prefix: %s", prefix)
		
		resp, err := client.ListStates(context.Background(), prefix)
		if err != nil {
			return fmt.Errorf("failed to list states: %w", err)
		}

		if len(resp.States) == 0 {
			fmt.Println("No states found")
			return nil
		}

		// Create tabwriter
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSIZE\tUPDATED\tLOCKED")
		
		for _, state := range resp.States {
			locked := ""
			if state.Locked {
				locked = "yes"
			}
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
				state.ID,
				state.Size,
				state.Updated.Format("2006-01-02 15:04:05"),
				locked,
			)
		}
		
		w.Flush()
		fmt.Printf("\nTotal: %d states\n", resp.Count)
		
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <state-id>",
	Short: "Show state metadata information",
	Aliases: []string{"show", "describe"},
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]

		printVerbose("Getting state metadata: %s", stateID)
		
		state, err := client.GetState(context.Background(), stateID)
		if err != nil {
			return fmt.Errorf("failed to get state info: %w", err)
		}

		// Pretty print as JSON
		data, _ := json.MarshalIndent(state, "", "  ")
		fmt.Println(string(data))
		
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "rm <state-id>",
	Short: "Delete a state",
	Aliases: []string{"delete", "remove"},
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]

		printVerbose("Deleting state: %s", stateID)
		
		err := client.DeleteState(context.Background(), stateID)
		if err != nil {
			return fmt.Errorf("failed to delete state: %w", err)
		}

		fmt.Printf("State deleted: %s\n", stateID)
		return nil
	},
}

var pullCmd = &cobra.Command{
	Use:   "pull <state-id> [output-file]",
	Short: "Download state data",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]
		
		printVerbose("Downloading state: %s", stateID)
		
		data, err := client.DownloadState(context.Background(), stateID)
		if err != nil {
			return fmt.Errorf("failed to download state: %w", err)
		}

		// Write to file or stdout
		if len(args) > 1 {
			outputFile := args[1]
			if err := os.WriteFile(outputFile, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			fmt.Printf("State downloaded to: %s\n", outputFile)
		} else {
			fmt.Print(string(data))
		}
		
		return nil
	},
}

var pushCmd = &cobra.Command{
	Use:   "push <state-id> <input-file>",
	Short: "Upload state data",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]
		inputFile := args[1]
		
		printVerbose("Uploading state: %s from %s", stateID, inputFile)
		
		// Read file
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Get lock ID if we have one
		lockID := getLockID(stateID)
		
		err = client.UploadState(context.Background(), stateID, data, lockID)
		if err != nil {
			return fmt.Errorf("failed to upload state: %w", err)
		}

		fmt.Printf("State uploaded: %s\n", stateID)
		return nil
	},
}

var lockCmd = &cobra.Command{
	Use:   "lock <state-id>",
	Short: "Lock a state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]
		
		printVerbose("Locking state: %s", stateID)
		
		lockInfo := &sdk.LockInfo{
			ID:      uuid.New().String(),
			Who:     fmt.Sprintf("taco@%s", getHostname()),
			Version: "1.0.0",
			Created: time.Now(),
		}
		
		result, err := client.LockState(context.Background(), stateID, lockInfo)
		if err != nil {
			return fmt.Errorf("failed to lock state: %w", err)
		}

		// Save lock ID locally
		saveLockID(stateID, result.ID)
		
		fmt.Printf("State locked: %s (lock ID: %s)\n", stateID, result.ID)
		return nil
	},
}

var unlockCmd = &cobra.Command{
	Use:   "unlock <state-id> [lock-id]",
	Short: "Unlock a state",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]
		
		lockID := ""
		if len(args) > 1 {
			lockID = args[1]
		} else {
			// Try to get from local storage
			lockID = getLockID(stateID)
			if lockID == "" {
				return fmt.Errorf("no lock ID provided and none found locally")
			}
		}
		
		printVerbose("Unlocking state: %s with lock ID: %s", stateID, lockID)
		
		err := client.UnlockState(context.Background(), stateID, lockID)
		if err != nil {
			return fmt.Errorf("failed to unlock state: %w", err)
		}

		// Remove local lock ID
		removeLockID(stateID)
		
		fmt.Printf("State unlocked: %s\n", stateID)
		return nil
	},
}

var acquireCmd = &cobra.Command{
	Use:   "acquire <state-id> [output-file]",
	Short: "Acquire state (pull + lock)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]
		
		printVerbose("Acquiring state: %s", stateID)
		
		// First lock
		lockInfo := &sdk.LockInfo{
			ID:      uuid.New().String(),
			Who:     fmt.Sprintf("taco@%s", getHostname()),
			Version: "1.0.0",
			Created: time.Now(),
		}
		
		result, err := client.LockState(context.Background(), stateID, lockInfo)
		if err != nil {
			return fmt.Errorf("failed to lock state: %w", err)
		}
		
		// Save lock ID
		saveLockID(stateID, result.ID)
		
		// Then download
		data, err := client.DownloadState(context.Background(), stateID)
		if err != nil {
			// Try to unlock on error
			client.UnlockState(context.Background(), stateID, result.ID)
			removeLockID(stateID)
			return fmt.Errorf("failed to download state: %w", err)
		}

		// Write to file or stdout
		if len(args) > 1 {
			outputFile := args[1]
			if err := os.WriteFile(outputFile, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			fmt.Printf("State acquired and saved to: %s (lock ID: %s)\n", outputFile, result.ID)
		} else {
			fmt.Print(string(data))
			fmt.Fprintf(os.Stderr, "\n[State acquired with lock ID: %s]\n", result.ID)
		}
		
		return nil
	},
}

var releaseCmd = &cobra.Command{
	Use:   "release <state-id> <input-file>",
	Short: "Release state (push + unlock)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sdk.NewClient(serverURL)
		stateID := args[0]
		inputFile := args[1]
		
		printVerbose("Releasing state: %s", stateID)
		
		// Get lock ID
		lockID := getLockID(stateID)
		if lockID == "" {
			return fmt.Errorf("no lock ID found for state %s - was it acquired?", stateID)
		}
		
		// Read file
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Upload with lock ID
		err = client.UploadState(context.Background(), stateID, data, lockID)
		if err != nil {
			return fmt.Errorf("failed to upload state: %w", err)
		}

		// Unlock
		err = client.UnlockState(context.Background(), stateID, lockID)
		if err != nil {
			return fmt.Errorf("failed to unlock state: %w", err)
		}

		// Remove local lock ID
		removeLockID(stateID)
		
		fmt.Printf("State released: %s\n", stateID)
		return nil
	},
}

// Lock ID management helpers

func getLockDir() string {
	return ".taco"
}

func getLockFile(stateID string) string {
	// Replace slashes to avoid path issues
	safeID := strings.ReplaceAll(stateID, "/", "_")
	return filepath.Join(getLockDir(), safeID+".lock")
}

func saveLockID(stateID, lockID string) {
	os.MkdirAll(getLockDir(), 0755)
	lockFile := getLockFile(stateID)
	os.WriteFile(lockFile, []byte(lockID), 0644)
}

func getLockID(stateID string) string {
	lockFile := getLockFile(stateID)
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func removeLockID(stateID string) {
	lockFile := getLockFile(stateID)
	os.Remove(lockFile)
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}