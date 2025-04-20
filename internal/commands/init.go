package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/database"
	"github.com/tildaslashalef/mindnest/internal/utils"
	"github.com/urfave/cli/v2"
)

// InitCommand returns the CLI command for initializing Mindnest
func InitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize or update Mindnest environment",
		Description: "Sets up the Mindnest environment including configuration directory " +
			"and database with necessary tables. Use this command for first-time setup " +
			"or to update your database schema after upgrading Mindnest to a new version.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Remove existing database before initialization",
				Value:   false,
			},
		},
		Action: func(c *cli.Context) error {
			utils.PrintHeading("Initializing Mindnest")

			// Get user's home directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				utils.PrintError(fmt.Sprintf("Failed to get user home directory: %s", err))
				return fmt.Errorf("failed to get user home directory: %w", err)
			}

			// Set up config directory (typically ~/.mindnest)
			configDir := filepath.Join(homeDir, ".mindnest")
			utils.PrintInfo("Configuration directory: " + color.YellowString("%s", configDir))

			// Ensure the directory exists
			if err := os.MkdirAll(configDir, 0755); err != nil {
				utils.PrintError(fmt.Sprintf("Failed to create config directory: %s", err))
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Extract the default environment file (with backup if needed)
			utils.PrintInfo("Extracting default configuration file")
			configFilePath := filepath.Join(configDir, ".env")

			// Use the SetupConfigDirectory function which will create a dated backup if .env exists
			if err := config.SetupConfigDirectory(configDir, true); err != nil {
				utils.PrintWarning(fmt.Sprintf("Failed to set up configuration files: %s", err))
				// Continue anyway as this is not critical
			}

			// Load configuration now that we've set up the directory and potentially
			// extracted the configuration file
			cfg, err := config.LoadFromEnv(configDir, configFilePath, true)
			if err != nil {
				utils.PrintError(fmt.Sprintf("Failed to load configuration: %s", err))
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Check if force flag is set
			if c.Bool("force") {
				utils.PrintWarning("Force flag detected, this will remove existing database")

				// Ask for confirmation
				fmt.Print("Are you sure you want to proceed? This cannot be undone. [y/N]: ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					utils.PrintWarning("Error reading input, assuming 'No'")
					return nil
				}

				if response != "y" && response != "Y" {
					utils.PrintInfo("Operation canceled by user")
					return nil
				}

				// Close any potential open database connections first
				if err := database.CloseDB(); err != nil {
					utils.PrintWarning(fmt.Sprintf("Failed to properly close database: %s", err))
					// Continue anyway as we're going to delete the files
				}

				// Delete main database file and all associated journal files
				if cfg.Database.Path != "" {
					dbPath := cfg.Database.Path
					utils.PrintWarning("Removing database files at: " + dbPath)

					// Remove main database file
					if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
						utils.PrintError(fmt.Sprintf("Failed to remove database file: %s", err))
					}

					// Also remove SQLite journal files
					os.Remove(dbPath + "-wal")
					os.Remove(dbPath + "-shm")

					utils.PrintSuccess("Existing database files removed successfully")
				}
			}

			// Initialize database directly with our loaded configuration
			utils.PrintInfo("Initializing new database...")
			if err := database.InitDB(cfg); err != nil {
				utils.PrintError(fmt.Sprintf("Failed to initialize database: %s", err))
				return fmt.Errorf("failed to initialize database: %w", err)
			}

			// Run migrations
			utils.PrintInfo("Applying database migrations...")
			migrationsApplied, err := database.RunMigrations()
			if err != nil {
				utils.PrintError(fmt.Sprintf("Failed to apply migrations: %s", err))
				return fmt.Errorf("failed to apply migrations: %w", err)
			}

			utils.PrintSuccess("✓ Mindnest initialized successfully!")

			// Display migration status
			if migrationsApplied > 0 {
				utils.PrintSuccess(fmt.Sprintf("Applied %d new migration(s)", migrationsApplied))
			} else {
				utils.PrintInfo("Database schema is already up-to-date")
			}

			utils.PrintInfo("Configuration file: " + color.YellowString("%s", configFilePath))
			utils.PrintInfo("Database location: " + color.YellowString("%s", cfg.Database.Path))
			utils.PrintInfo("Log file location: " + color.YellowString("%s", cfg.Logging.Output))
			fmt.Println("")
			utils.PrintInfo("You can now use " + color.CyanString("mindnest") + " to review your code.")

			return nil
		},
	}
}
