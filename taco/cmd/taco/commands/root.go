package commands

import (
    "fmt"
    "os"
    "github.com/spf13/cobra"
)


type Config struct {
    ServerUrl   string  `json:"server_url"`
}


var (
    // Global flags
    serverURL string
    verbose   bool

    globalConfig *Config

    // rootCmd represents the base command
    rootCmd = &cobra.Command{
        Use:   "taco",
        Short: "OpenTaco CLI - Terraform state management",
        Long: `OpenTaco CLI provides command-line access to the OpenTaco state management service.

It allows you to manage Terraform states, handle locking, and perform state operations
through a simple CLI interface.`,
        PersistentPreRunE: func(cmd  *cobra.Command, args []string) error {
            if cmd.Name() == "setup" {
                return nil
            }

            config, err := loadOrCreateConfig() 

            if err != nil {
                return fmt.Errorf("Failed to load configuration: %w", err)
            }

            globalConfig = config 

            // Prioritize environment variables over config file 
            // env > flags > config
            if !cmd.Flag("server").Changed && config.ServerUrl != "" {
                serverURL = config.ServerUrl
            }

            return nil 
        },
    }
)

// Execute adds all child commands to the root command and runs it.
func Execute() error { return rootCmd.Execute() }

func init() {
    // Global flags
    rootCmd.PersistentFlags().StringVar(&serverURL, "server", getEnvOrDefault("OPENTACO_SERVER", "http://localhost:8080"), "OpenTaco server URL")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}




// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

// printVerbose prints a message if verbose mode is enabled
func printVerbose(format string, args ...interface{}) {
    if verbose {
        fmt.Printf("[DEBUG] "+format+"\n", args...)
    }
}




