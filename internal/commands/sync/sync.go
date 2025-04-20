package sync

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/utils"
	"github.com/urfave/cli/v2"
)

// SyncCommand returns the CLI command for syncing data to the server
func SyncCommand() *cli.Command {
	return &cli.Command{
		Name:        "sync",
		Usage:       "Sync local data with the Mindnest web server",
		Description: "Sync workspaces, reviews, and issues to the Mindnest web server",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Simulate sync without actually sending data to server",
				Value: false,
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:        "account",
				Usage:       "Manage server account connection",
				Description: "Link or unlink your local CLI with your Mindnest web account",
				Subcommands: []*cli.Command{
					{
						Name:        "link",
						Usage:       "Link to web account",
						Description: "Connect your local CLI to your Mindnest web account",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "token",
								Usage:    "Personal access token from the web interface",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "name",
								Usage: "A name for this CLI instance (e.g., 'Work Laptop')",
							},
						},
						Action: linkAccountAction,
					},
					{
						Name:        "unlink",
						Usage:       "Unlink from web account",
						Description: "Remove the connection to your Mindnest web account",
						Action:      unlinkAccountAction,
					},
					{
						Name:        "status",
						Usage:       "Check account connection status",
						Description: "Verify if your CLI is connected to a Mindnest web account",
						Action:      accountStatusAction,
					},
				},
			},
			{
				Name:        "status",
				Usage:       "Show sync status",
				Description: "Display status of the last sync operations",
				Action:      syncStatusAction,
			},
			{
				Name:        "config",
				Usage:       "Configure sync settings",
				Description: "Modify sync configuration settings",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "server",
						Usage: "Server URL for syncing",
					},
					&cli.StringFlag{
						Name:  "token",
						Usage: "Personal access token for syncing",
					},
					&cli.StringFlag{
						Name:  "device-name",
						Usage: "Device name for syncing",
					},
					&cli.BoolFlag{
						Name:  "enabled",
						Usage: "Enable or disable syncing",
					},
				},
				Action: syncConfigAction,
			},
		},
		Action: syncAction,
	}
}

// syncAction is the main action for the sync command
func syncAction(c *cli.Context) error {
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	isDryRun := c.Bool("dry-run")

	if !application.Sync.IsConfigured() && !isDryRun {
		// TODO: Consider moving this check into the TUI start sequence?
		utils.PrintError("Sync is not configured. Use 'mindnest sync account link --token <token>' to configure")
		return fmt.Errorf("sync not configured")
	}

	loggy.Info("Starting manual sync TUI", "dry_run", isDryRun)

	// Create the TUI model using the constructor from model.go
	model := NewModel(application, isDryRun)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(model) // Removed model initialization here, it's done in model.Init()
	if _, err := p.Run(); err != nil {
		loggy.Error("Error running sync TUI", "error", err)
		return fmt.Errorf("error running sync UI: %w", err)
	}

	loggy.Info("Sync TUI finished.")
	return nil
}

// linkAccountAction handles linking to a web account
func linkAccountAction(c *cli.Context) error {
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	// Get token from flag
	token := c.String("token")
	if token == "" {
		return fmt.Errorf("token is required")
	}

	// Get device name from flag
	deviceName := c.String("name")
	if deviceName == "" {
		deviceName = utils.GenerateProjectName()
	}

	// Set token
	if err := application.Sync.SetToken(token); err != nil {
		return fmt.Errorf("setting token: %w", err)
	}

	// Set device name in config
	application.Config.Server.DeviceName = deviceName
	application.Config.Server.Enabled = true

	// Save other settings
	ctx := c.Context
	if application.Settings != nil {
		if err := application.Settings.SetSetting(ctx, "sync.server_url", application.Config.Server.URL); err != nil {
			loggy.Warn("Failed to save server URL to settings", "error", err)
		}
		if err := application.Settings.SetSetting(ctx, "sync.device_name", deviceName); err != nil {
			loggy.Warn("Failed to save device name to settings", "error", err)
		}
		if err := application.Settings.SetSetting(ctx, "sync.enabled", "true"); err != nil {
			loggy.Warn("Failed to save enabled status to settings", "error", err)
		}
	}

	// Verify token
	valid, err := application.Sync.VerifyToken(ctx)
	if err != nil {
		return fmt.Errorf("verifying token: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid token")
	}

	utils.PrintSuccess("Successfully linked to Mindnest Web as " + application.Config.Server.DeviceName)
	return nil
}

// unlinkAccountAction handles unlinking from a web account
func unlinkAccountAction(c *cli.Context) error {
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	// Remove token
	if err := application.Sync.SetToken(""); err != nil {
		return fmt.Errorf("removing token: %w", err)
	}

	// Disable sync
	application.Config.Server.Enabled = false

	// Update enabled setting
	if application.Settings != nil {
		ctx := c.Context
		if err := application.Settings.SetSetting(ctx, "sync.enabled", "false"); err != nil {
			loggy.Warn("Failed to save enabled status to settings", "error", err)
		}
	}

	utils.PrintSuccess("Successfully unlinked from Mindnest Web")
	return nil
}

// accountStatusAction handles checking account status
func accountStatusAction(c *cli.Context) error {
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	// Check if configured
	if !application.Sync.IsConfigured() {
		utils.PrintError("Not linked to Mindnest Web")
		return nil
	}

	// Verify token
	valid, err := application.Sync.VerifyToken(c.Context)
	if err != nil {
		loggy.Warn("Error verifying token", "error", err)
	}

	if valid {
		utils.PrintHeading("Account Linked")
		utils.PrintKeyValueWithColor("Server URL", application.Config.Server.URL, utils.Theme.Info)
		utils.PrintKeyValueWithColor("Device Name", application.Config.Server.DeviceName, utils.Theme.Info)
	} else {
		utils.PrintError("Token is invalid or expired")
	}

	return nil
}

// syncStatusAction handles showing sync status
func syncStatusAction(c *cli.Context) error {
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	// Check if configured
	if !application.Sync.IsConfigured() {
		fmt.Println("Sync is not configured")
		return nil
	}

	// Define table columns
	tableHeaders := []string{"Type", "Entity", "ID", "Status", "Error", "Started", "Completed"}
	tableRows := [][]string{}

	// Get all sync logs (using 0, 0 to get all records)
	syncLogs, err := application.Sync.GetSyncLogs(c.Context, "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("error getting sync status: %w", err)
	}

	// Format time to be more readable
	formatTime := func(t time.Time) string {
		if t.IsZero() {
			return "-"
		}
		return t.Format("Jan 02 15:04:05")
	}

	// Format success status
	formatSuccess := func(success bool) string {
		if success {
			return "✓ Success"
		}
		return "✗ Failed"
	}

	// Truncate long strings
	truncate := func(s string, maxLen int) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen-3] + "..."
	}

	for _, log := range syncLogs {
		tableRows = append(tableRows, []string{
			string(log.SyncType),
			string(log.EntityType),
			log.EntityID,
			formatSuccess(log.Success),
			truncate(log.ErrorMessage, 64),
			formatTime(log.StartedAt),
			formatTime(log.CompletedAt),
		})
	}

	// Use paginated table with 10 items per page for all record sets
	utils.PrintPaginatedTable(tableHeaders, tableRows, 20, "Sync Logs")

	return nil
}

// syncConfigAction handles configuring sync settings
func syncConfigAction(c *cli.Context) error {
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	// Update server URL if provided
	if c.IsSet("server") {
		serverURL := c.String("server")
		application.Config.Server.URL = serverURL

		utils.PrintKeyValueWithColor("Server URL Updated", serverURL, utils.Theme.Info)

		// Save to settings
		if application.Settings != nil {
			ctx := c.Context
			if err := application.Settings.SetSetting(ctx, "sync.server_url", serverURL); err != nil {
				loggy.Warn("Failed to save server URL to settings", "error", err)
			}
		}
	}

	if c.IsSet("token") {
		token := c.String("token")

		if application.Settings != nil {
			ctx := c.Context
			if err := application.Settings.SetSetting(ctx, "sync.server_token", token); err != nil {
				loggy.Warn("Failed to save token to settings", "error", err)
			}
		}

		utils.PrintKeyValueWithColor("Token Updated", token, utils.Theme.Info)
	}

	if c.IsSet("device-name") {
		deviceName := c.String("device-name")
		if application.Settings != nil {
			ctx := c.Context
			if err := application.Settings.SetSetting(ctx, "sync.device_name", deviceName); err != nil {
				loggy.Warn("Failed to save device name to settings", "error", err)
			}
		}

		utils.PrintKeyValueWithColor("Device Name Updated", deviceName, utils.Theme.Info)

	}

	// Update enabled status if provided
	if c.IsSet("enabled") {
		enabled := c.Bool("enabled")
		application.Config.Server.Enabled = enabled

		// Save to settings
		if application.Settings != nil {
			ctx := c.Context
			enabledStr := "false"
			if enabled {
				enabledStr = "true"
			}
			if err := application.Settings.SetSetting(ctx, "sync.enabled", enabledStr); err != nil {
				loggy.Warn("Failed to save enabled status to settings", "error", err)
			}
		}

		utils.PrintKeyValueWithColor("Sync enabled", fmt.Sprintf("%v", enabled), utils.Theme.Info)
	}

	// Display current config if no changes were made
	if !c.IsSet("server") && !c.IsSet("enabled") && !c.IsSet("token") && !c.IsSet("device-name") {
		utils.PrintHeading("Current Sync Configuration")
		utils.PrintKeyValueWithColor("Server URL", application.Config.Server.URL, utils.Theme.Info)
		utils.PrintKeyValueWithColor("Device Name", application.Config.Server.DeviceName, utils.Theme.Info)
		utils.PrintKeyValueWithColor("Sync enabled", fmt.Sprintf("%v", application.Config.Server.Enabled), utils.Theme.Info)
	}

	return nil
}
