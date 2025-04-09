package cli

import (
	"log/slog"
	"os"

	"github.com/go-substrate/strate/backend/app"
	"github.com/go-substrate/strate/backend/config"
	"github.com/go-substrate/strate/backend/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Strate backend server",
	Long: `Start the Strate backend server with the specified configuration.
This will start the HTTP server and make it available for client connections.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		// Bind flags to viper
		viper.BindPFlag("log.level", cmd.Flags().Lookup("log-level"))
		viper.BindPFlag("log.format", cmd.Flags().Lookup("log-format"))

		// Handle verbose flag specially
		if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
			viper.Set("log.level", "debug")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")

		// Load configuration
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			slog.Error("Failed to load configuration", "error", err)
			os.Exit(1)
		}

		// Update logging configuration based on flags
		// Since flags were bound to viper in PreRun, they're already reflected in cfg
		// But we need to reconfigure the logger
		if cmd.Flags().Changed("log-level") || cmd.Flags().Changed("verbose") ||
			cmd.Flags().Changed("log-format") {
			log.Configure(cfg.Log)
		}

		// Set environment variables based on config for backward compatibility
		config.SetEnvFromConfig(cfg)

		// Create and start the application
		app, err := app.NewApp(cfg)
		if err != nil {
			slog.Error("Failed to create application", "error", err)
			os.Exit(1)
		}

		// Start the server
		if err := app.Serve(); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("config", "", "config file (default is ./config.yaml)")

	// Add log-related flags
	serveCmd.Flags().StringP("log-level", "l", "", "Log level (debug, info, warn, error)")
	serveCmd.Flags().String("log-format", "", "Log format (json, text)")
	serveCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging (sets log level to debug)")
}
