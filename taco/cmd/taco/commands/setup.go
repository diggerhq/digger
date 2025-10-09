package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run the configuration setup wizard",
	Long:  "Run the configuration setup wizard to configure your OpenTaco CLI settings.",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("OpenTaco CLI Configuration Setup")
	fmt.Println("This will update your configuration settings.\n")

	config, err := runSetupWizard()

	if err != nil {
		return err
	}

	if err := saveConfig(config); err != nil {
		return fmt.Errorf("Failed to save configuration %w", err)
	}

	fmt.Println("Configuration Updated Successfully")
	return nil
}

// return the configuration location
func configPath() (string, error) {
	home, err := os.UserHomeDir()

	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "opentaco")

	if err := os.MkdirAll(dir, 0o755); err != nil {
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
		fmt.Println()
		fmt.Println()
		fmt.Println("  /$$$$$$                             /$$$$$$$$ /$$$$$$   /$$$$$$   /$$$$$$ ")
		fmt.Println(" /$$__  $$                           |__  $$__//$$__  $$ /$$__  $$ /$$__  $$")
		fmt.Println("| $$  \\ $$  /$$$$$$   /$$$$$$  /$$$$$$$ | $$  | $$  \\ $$| $$  \\__/| $$  \\ $$")
		fmt.Println("| $$  | $$ /$$__  $$ /$$__  $$| $$__  $$| $$  | $$$$$$$$| $$      | $$  | $$")
		fmt.Println("| $$  | $$| $$  \\ $$| $$$$$$$$| $$  \\ $$| $$  | $$__  $$| $$      | $$  | $$")
		fmt.Println("| $$  | $$| $$  | $$| $$_____/| $$  | $$| $$  | $$  | $$| $$    $$| $$  | $$")
		fmt.Println("|  $$$$$$/| $$$$$$$/|  $$$$$$$| $$  | $$| $$  | $$  | $$|  $$$$$$/|  $$$$$$/")
		fmt.Println(" \\______/ | $$____/  \\_______/|__/  |__/|__/  |__/  |__/ \\______/  \\______/ ")
		fmt.Println("          | $$                                                              ")
		fmt.Println("          | $$                                                              ")
		fmt.Println("          |__/                                                              ")
		fmt.Println("")
		fmt.Println("🌮 Welcome to OpenTaco CLI! 🌮")
		fmt.Println("It looks like it's your first time running the CLI.")
		fmt.Println("Let's set up your configuration.\n")

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

		fmt.Print("Enter OpenTaco server url [http://localhost:8080]: ")
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

		if confirm == "" || confirm == "y" || confirm == "yes" {
			return config, nil
		} else if confirm == "n" || confirm == "no" {
			fmt.Println("Configuration cancelled")
			os.Exit(0)
		} else {
			fmt.Println("Please enter 'y' for yes or 'n' for no.")
		}
	}

}

func GetGlobalConfig() *Config {
	return globalConfig
}
