package commands

import (
    "fmt"
    "os"
    "encoding/json"
    "path/filepath"
    "strings" 
    "bufio"
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


// return the configuration location
func configPath() (string, error) {
    home, err := os.UserHomeDir() 
    
    if err != nil { 
        return "", err
    }
    dir := filepath.Join(home, ".config", "opentaco")
    if err := os.MkdirAll(dir, 0o755); err != nil  {
        return "", err 
    }
    return filepath.Join(dir, "config.json"), nil 

}


// loads and returns the config 
func loadConfig() (*Config, error) { 
    path, err := configPath()
    
    if err != nil {
        return nil, err 
    }

    data, err := os.ReadFile(path) 
    
    if os.IsNotExist(err) {
        return nil, nil // config file doesn't exist 
    }

    if err != nil {
        return nil, err 
    }
    
    var config Config
    
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err 
    }

    return &config, nil 
}




// saves the configuration to the path
func saveConfig(config *Config) error {
    path, err := configPath() 
    
    if err != nil {
        return err 
    }

    data, err := json.MarshalIndent(config, "", " ")

    if err != nil {
        return err 
    }

    return os.WriteFile(path, data, 0o600) 
}




func loadOrCreateConfig() (*Config, error) {
    config, err := loadConfig()
    
    if err != nil {
        return nil, err 
    }

    // You dont have a config, start the wizard experience 
    if config == nil {
        fmt.Println("Welcome to OpenTaco CLI!")
        fmt.Println("It looks like its your first time running the CLI.")
        fmt.Println("Let's setup your configuration.\n")
    
        config, err = runSetupWizard()

        if err != nil {
            return nil, err
        }

        if err := saveConfig(config); err != nil {
            return nil, fmt.Errorf("Failed to save configuration: %w", err)
        }

        fmt.Println("Configuration saved successfully!")
        fmt.Println("You can reconfigure anytime by running: taco setup\n")

    }


    return config, nil 

}



func runSetupWizard() (*Config, error) { 
    reader := bufio.NewReader(os.Stdin)
    config := &Config{} 

    // Get server url 

    for {

        fmt.Print("Enter OpenTaco server url [http://localhost:8080]:")
        serverURL, err := reader.ReadString('\n')
        if err != nil {
            return nil, err 
        }

        serverURL = strings.TrimSpace(serverURL) 
        if serverURL == "" {
            serverURL = "http://localhost:8080"
        }

        config.ServerUrl = serverURL

        break
    }


    fmt.Println("Configuration Summary:")
    fmt.Printf("    Server URL: %s\n", config.ServerUrl)

    for {
        fmt.Print("\nSave this configuration? [Y/n]: ")
        confirm, err := reader.ReadString('\n')
        if err != nil {
            return nil, err 
        }
    
        confirm = strings.ToLower(strings.TrimSpace(confirm)) 

        if confirm == "" || confirm == "y" || confirm == "yes"{
            return config, nil 
        } else if confirm == "n" || confirm == "no" {
            fmt.Println("Configuration cancelled")
            os.Exit(0) 
        } else {
            fmt.Println("Please enter 'y' for yes or 'n' for no.")
        }
    }

}


func getConfigurationValue(flagValue, configValue, envKey, defaultValue string) string {
    if envValue := os.Getenv(envKey); envValue != "" {
        return envValue 
    }
    if flagValue != "" {
        return flagValue

    }

    if configValue != "" {
        return configValue
    }

    return defaultValue

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



func GetGlobalConfig() *Config  {
    return globalConfig 
}


