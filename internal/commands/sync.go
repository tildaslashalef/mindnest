package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"

	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/sync"
	"github.com/tildaslashalef/mindnest/internal/tui"
	"github.com/tildaslashalef/mindnest/internal/utils"
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
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	isDryRun := c.Bool("dry-run")

	// Check if sync is configured
	if !application.Sync.IsConfigured() && !isDryRun {
		return fmt.Errorf("sync is not configured. Use 'mindnest sync account link --token <token>' to configure")
	}

	// Log that we're starting a sync
	loggy.Info("Starting manual sync", "dry_run", isDryRun)

	// Create and initialize the bubbletea model
	model := NewSyncModel(application, isDryRun)
	p := tea.NewProgram(model)

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running sync UI: %w", err)
	}

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

// SyncKeyMap defines keybindings for the sync TUI
type SyncKeyMap struct {
	Help  key.Binding
	Quit  key.Binding
	Enter key.Binding
}

// ShortHelp returns keybindings to show in the mini help view
func (k SyncKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Enter}
}

// FullHelp returns all keybindings for the help view
func (k SyncKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit, k.Enter},
	}
}

// DefaultSyncKeyMap returns the default keybindings
func DefaultSyncKeyMap() SyncKeyMap {
	return SyncKeyMap{
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
	}
}

// Message types
type (
	// SyncStartMsg is the initial message sent to the model
	SyncStartMsg struct{}

	// SyncProgressMsg is sent to update sync progress
	SyncProgressMsg struct {
		EntityType    sync.EntityType
		EntityID      string
		CurrentItem   int
		TotalItems    int
		ItemsSynced   int
		ItemsFailed   int
		ErrorMessage  string
		ProgressValue float64
	}

	// SyncCompleteMsg is sent when sync is complete
	SyncCompleteMsg struct {
		Result          *sync.SyncResult
		Error           error
		WorkspaceCount  int
		ReviewCount     int
		ReviewFileCount int
		IssueCount      int
		FileCount       int
	}
)

// SyncModel represents the state of the sync TUI
type SyncModel struct {
	app      *app.App
	dryRun   bool
	keymap   SyncKeyMap
	help     help.Model
	spinner  spinner.Model
	progress progress.Model
	styles   tui.Styles

	// UI state
	ready        bool
	loading      bool
	showHelp     bool
	error        string
	status       string
	width        int
	height       int
	lastProgress SyncProgressMsg
	result       *sync.SyncResult
	syncing      bool
	currentStage string
	entityTypes  []sync.EntityType

	// Item counts by type
	workspaceCount  int
	reviewCount     int
	reviewFileCount int
	issueCount      int
	fileCount       int
}

// NewSyncModel creates a new sync model
func NewSyncModel(a *app.App, dryRun bool) SyncModel {
	keymap := DefaultSyncKeyMap()
	help := help.New()

	s := spinner.New()
	s.Spinner = spinner.Dot

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	return SyncModel{
		app:         a,
		dryRun:      dryRun,
		keymap:      keymap,
		help:        help,
		spinner:     s,
		progress:    p,
		styles:      tui.DefaultStyles(),
		loading:     false,
		syncing:     false,
		entityTypes: []sync.EntityType{sync.EntityTypeWorkspace, sync.EntityTypeReview, sync.EntityTypeReviewFile, sync.EntityTypeIssue, sync.EntityTypeFile},
	}
}

// Init initializes the model
func (m SyncModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return SyncStartMsg{}
		},
	)
}

// startSync begins the sync process
func (m SyncModel) startSync() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		if m.dryRun {
			return m.simulateSync(ctx)
		} else {
			return m.performRealSync(ctx)
		}
	}
}

// simulateSync simulates the sync process for dry-run mode
func (m SyncModel) simulateSync(ctx context.Context) tea.Msg {
	// Simulate sync process with artificial delays
	result := &sync.SyncResult{
		TotalItems:   0,
		SuccessItems: 0,
		FailedItems:  0,
		Duration:     0,
		Success:      true,
	}

	start := time.Now()

	// Get all entities that need syncing (unsynced + previously failed)
	workspaceCount, reviewCount, reviewFileCount, issueCount, fileCount,
		unsyncedWorkspaces, unsyncedReviews, unsyncedReviewFiles, unsyncedIssues, unsyncedFiles, totalEntitiesToSync :=
		m.getEntitiesToSync(ctx)

	// If nothing to sync, return empty result
	if totalEntitiesToSync == 0 {
		return SyncCompleteMsg{
			Result: &sync.SyncResult{
				TotalItems:   0,
				SuccessItems: 0,
				FailedItems:  0,
				Duration:     time.Since(start),
				Success:      true,
			},
			WorkspaceCount:  workspaceCount,
			ReviewCount:     reviewCount,
			ReviewFileCount: reviewFileCount,
			IssueCount:      issueCount,
			FileCount:       fileCount,
			Error:           nil,
		}
	}

	result.TotalItems = totalEntitiesToSync
	loggy.Info("Total items to sync", "count", totalEntitiesToSync)

	// Store counts for display
	m.workspaceCount = workspaceCount
	m.reviewCount = reviewCount
	m.reviewFileCount = reviewFileCount
	m.issueCount = issueCount
	m.fileCount = fileCount

	// Create tracking variables for simulation
	currentProgress := 0

	// Channel to send progress updates back to the UI (unused in dry-run)
	progCh := make(chan tea.Msg)
	go func() {
		for range progCh {
			// In a real implementation, you would use tea.Program.Send() here
		}
	}()

	// 1. Simulate syncing workspaces
	for i, wsID := range unsyncedWorkspaces {
		time.Sleep(50 * time.Millisecond) // Simulate network delay
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntitiesToSync)

		// All workspace syncs succeed
		result.SuccessItems++

		progMsg := SyncProgressMsg{
			EntityType:    sync.EntityTypeWorkspace,
			EntityID:      wsID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedWorkspaces),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   result.FailedItems,
			ErrorMessage:  "",
			ProgressValue: progress,
		}

		// This is for dry-run mode - in real implementation we would send to tea program
		m.lastProgress = progMsg
		m.currentStage = fmt.Sprintf("Syncing %s: %d/%d", progMsg.EntityType, i+1, len(unsyncedWorkspaces))
		m.progress.SetPercent(progress)
	}

	// 2. Simulate syncing reviews
	for i, reviewID := range unsyncedReviews {
		time.Sleep(50 * time.Millisecond) // Simulate network delay
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntitiesToSync)

		// All review syncs succeed
		result.SuccessItems++

		progMsg := SyncProgressMsg{
			EntityType:    sync.EntityTypeReview,
			EntityID:      reviewID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedReviews),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   result.FailedItems,
			ErrorMessage:  "",
			ProgressValue: progress,
		}

		// This is for dry-run mode - in real implementation we would send to tea program
		m.lastProgress = progMsg
		m.currentStage = fmt.Sprintf("Syncing %s: %d/%d", progMsg.EntityType, i+1, len(unsyncedReviews))
		m.progress.SetPercent(progress)
	}

	// 3. Simulate syncing review files
	for i, fileID := range unsyncedReviewFiles {
		time.Sleep(30 * time.Millisecond) // Faster for smaller entities
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntitiesToSync)

		// All review file syncs succeed
		result.SuccessItems++

		progMsg := SyncProgressMsg{
			EntityType:    sync.EntityTypeReviewFile,
			EntityID:      fileID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedReviewFiles),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   result.FailedItems,
			ErrorMessage:  "",
			ProgressValue: progress,
		}

		// This is for dry-run mode - in real implementation we would send to tea program
		m.lastProgress = progMsg
		m.currentStage = fmt.Sprintf("Syncing %s: %d/%d", progMsg.EntityType, i+1, len(unsyncedReviewFiles))
		m.progress.SetPercent(progress)
	}

	// 4. Simulate syncing issues
	for i, issueID := range unsyncedIssues {
		time.Sleep(30 * time.Millisecond) // Faster for smaller entities
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntitiesToSync)

		// All issues syncs succeed
		result.SuccessItems++

		progMsg := SyncProgressMsg{
			EntityType:    sync.EntityTypeIssue,
			EntityID:      issueID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedIssues),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   result.FailedItems,
			ErrorMessage:  "",
			ProgressValue: progress,
		}

		// This is for dry-run mode - in real implementation we would send to tea program
		m.lastProgress = progMsg
		m.currentStage = fmt.Sprintf("Syncing %s: %d/%d", progMsg.EntityType, i+1, len(unsyncedIssues))
		m.progress.SetPercent(progress)
	}

	// 5. Simulate syncing files
	for i, fileID := range unsyncedFiles {
		time.Sleep(30 * time.Millisecond) // Faster for smaller entities
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntitiesToSync)

		// All file syncs succeed
		result.SuccessItems++

		progMsg := SyncProgressMsg{
			EntityType:    sync.EntityTypeFile,
			EntityID:      fileID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedFiles),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   result.FailedItems,
			ErrorMessage:  "",
			ProgressValue: progress,
		}

		// This is for dry-run mode - in real implementation we would send to tea program
		m.lastProgress = progMsg
		m.currentStage = fmt.Sprintf("Syncing %s: %d/%d", progMsg.EntityType, i+1, len(unsyncedFiles))
		m.progress.SetPercent(progress)
	}

	close(progCh)
	result.Duration = time.Since(start)

	// All operations succeed, so no need to log failures
	result.Success = true

	// Store the breakdown in the result
	result.WorkspaceCount = workspaceCount
	result.ReviewCount = reviewCount
	result.ReviewFileCount = reviewFileCount
	result.IssueCount = issueCount
	result.FileCount = fileCount

	return SyncCompleteMsg{
		Result:          result,
		Error:           nil,
		WorkspaceCount:  workspaceCount,
		ReviewCount:     reviewCount,
		ReviewFileCount: reviewFileCount,
		IssueCount:      issueCount,
		FileCount:       fileCount,
	}
}

// performRealSync performs the actual sync with the server
func (m SyncModel) performRealSync(ctx context.Context) tea.Msg {
	// Setup a goroutine to watch for changes in the DB and update progress
	progCh := make(chan tea.Msg)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Get all entities that need syncing (unsynced + previously failed)
	workspaceCount, reviewCount, reviewFileCount, issueCount, fileCount,
		_, _, _, _, _, totalToSync := m.getEntitiesToSync(ctx)

	// If nothing to sync, return empty result immediately
	if totalToSync == 0 {
		return SyncCompleteMsg{
			Result: &sync.SyncResult{
				TotalItems:      0,
				SuccessItems:    0,
				FailedItems:     0,
				Duration:        time.Millisecond * 2, // Minimal time
				Success:         true,
				WorkspaceCount:  0,
				ReviewCount:     0,
				ReviewFileCount: 0,
				IssueCount:      0,
				FileCount:       0,
			},
			WorkspaceCount:  0,
			ReviewCount:     0,
			ReviewFileCount: 0,
			IssueCount:      0,
			FileCount:       0,
			Error:           nil,
		}
	}

	go func() {
		defer close(progCh)

		// Real sync doesn't provide progress updates currently, so we'll simulate them
		// In a real implementation, we could have the sync service emit events we can listen to
		synced := 0

		// Poll for changes in synced items
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				synced++
				if synced > totalToSync {
					return
				}

				progress := float64(synced) / float64(totalToSync)
				progMsg := SyncProgressMsg{
					EntityType:    sync.EntityTypeWorkspace,
					CurrentItem:   synced,
					TotalItems:    totalToSync,
					ItemsSynced:   synced,
					ProgressValue: progress,
				}

				// This would use tea.Program.Send() in a real implementation
				m.lastProgress = progMsg
				m.currentStage = fmt.Sprintf("Syncing: %d/%d items", synced, totalToSync)
				m.progress.SetPercent(progress)
			}
		}
	}()

	// Perform actual sync
	result, err := m.app.Sync.SyncAll(ctx)
	cancel() // Stop the progress update goroutine

	return SyncCompleteMsg{
		Result:          result,
		Error:           err,
		WorkspaceCount:  workspaceCount,
		ReviewCount:     reviewCount,
		ReviewFileCount: reviewFileCount,
		IssueCount:      issueCount,
		FileCount:       fileCount,
	}
}

// getEntitiesToSync collects all entities (both unsynced and previously failed) that need to be synced
func (m SyncModel) getEntitiesToSync(ctx context.Context) (
	workspaceCount int, reviewCount int, reviewFileCount int, issueCount int, fileCount int,
	unsyncedWorkspaces []string, unsyncedReviews []string, unsyncedReviewFiles []string, unsyncedIssues []string, unsyncedFiles []string,
	totalEntitiesToSync int) {

	// Reset counts
	workspaceCount = 0
	reviewCount = 0
	reviewFileCount = 0
	issueCount = 0
	fileCount = 0
	totalEntitiesToSync = 0

	// 1. Workspaces
	unsyncedWorkspaces, err := m.app.Sync.GetUnsyncedWorkspaces(ctx, 100)
	if err != nil {
		loggy.Error("Error getting unsynced workspaces", "error", err)
	}

	// Get failed workspaces
	failedWorkspaces, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeWorkspace, 100)
	if err != nil {
		loggy.Error("Error getting failed workspace sync logs", "error", err)
	}

	// Combine both lists (ensuring no duplicates)
	workspacesMap := make(map[string]bool)
	for _, id := range unsyncedWorkspaces {
		workspacesMap[id] = true
	}
	for _, id := range failedWorkspaces {
		workspacesMap[id] = true
	}

	// Convert map back to slice
	unsyncedWorkspaces = []string{}
	for id := range workspacesMap {
		unsyncedWorkspaces = append(unsyncedWorkspaces, id)
	}

	workspaceCount = len(unsyncedWorkspaces)
	totalEntitiesToSync += workspaceCount

	// 2. Reviews
	reviewsMap := make(map[string]bool) // Use map to avoid duplicates

	// Get reviews associated with workspaces
	for _, wsID := range unsyncedWorkspaces {
		reviews, err := m.app.Sync.GetUnsyncedReviews(ctx, wsID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced reviews", "error", err, "workspace_id", wsID)
		}

		// Add to map to avoid duplicates
		for _, reviewID := range reviews {
			reviewsMap[reviewID] = true
		}
	}

	// Get failed reviews
	failedReviews, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeReview, 100)
	if err != nil {
		loggy.Error("Error getting failed review sync logs", "error", err)
	}

	// Add failed reviews to map
	for _, reviewID := range failedReviews {
		reviewsMap[reviewID] = true
	}

	// Convert map to slice
	for reviewID := range reviewsMap {
		unsyncedReviews = append(unsyncedReviews, reviewID)
	}

	reviewCount = len(unsyncedReviews)
	totalEntitiesToSync += reviewCount

	// 3. Review Files
	reviewFilesMap := make(map[string]bool) // Use map to avoid duplicates

	for _, reviewID := range unsyncedReviews {
		files, err := m.app.Sync.GetUnsyncedReviewFiles(ctx, reviewID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced review files", "error", err, "review_id", reviewID)
		}

		// Add to map to avoid duplicates
		for _, fileID := range files {
			reviewFilesMap[fileID] = true
		}
	}

	// Get failed review files
	failedReviewFiles, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeReviewFile, 100)
	if err != nil {
		loggy.Error("Error getting failed review file sync logs", "error", err)
	}

	// Add failed review files to map
	for _, fileID := range failedReviewFiles {
		reviewFilesMap[fileID] = true
	}

	// Convert map to slice
	for fileID := range reviewFilesMap {
		unsyncedReviewFiles = append(unsyncedReviewFiles, fileID)
	}

	reviewFileCount = len(unsyncedReviewFiles)
	totalEntitiesToSync += reviewFileCount

	// 4. Issues
	issuesMap := make(map[string]bool) // Use map to avoid duplicates

	for _, reviewID := range unsyncedReviews {
		issues, err := m.app.Sync.GetUnsyncedIssues(ctx, reviewID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced issues", "error", err, "review_id", reviewID)
		}

		// Add to map to avoid duplicates
		for _, issueID := range issues {
			issuesMap[issueID] = true
		}
	}

	// Also get directly modified issues
	directIssues, err := m.app.Sync.GetUnsyncedIssues(ctx, "", 100)
	if err != nil {
		loggy.Error("Error getting directly modified issues", "error", err)
	} else {
		// Add to map to avoid duplicates
		for _, issueID := range directIssues {
			issuesMap[issueID] = true
		}
	}

	// Get failed issues
	failedIssues, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeIssue, 100)
	if err != nil {
		loggy.Error("Error getting failed issue sync logs", "error", err)
	}

	// Add failed issues to map
	for _, issueID := range failedIssues {
		issuesMap[issueID] = true
	}

	// Convert map to slice
	for issueID := range issuesMap {
		unsyncedIssues = append(unsyncedIssues, issueID)
	}

	issueCount = len(unsyncedIssues)
	totalEntitiesToSync += issueCount

	// 5. Files
	filesMap := make(map[string]bool) // Use map to avoid duplicates

	for _, workspaceID := range unsyncedWorkspaces {
		files, err := m.app.Sync.GetUnsyncedFiles(ctx, workspaceID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced files", "error", err, "workspace_id", workspaceID)
		}

		// Add to map to avoid duplicates
		for _, fileID := range files {
			filesMap[fileID] = true
		}
	}

	// Also get directly modified files
	directFiles, err := m.app.Sync.GetUnsyncedFiles(ctx, "", 100)
	if err != nil {
		loggy.Error("Error getting directly modified files", "error", err)
	} else {
		// Add to map to avoid duplicates
		for _, fileID := range directFiles {
			filesMap[fileID] = true
		}
	}

	// Get failed files
	failedFiles, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeFile, 100)
	if err != nil {
		loggy.Error("Error getting failed file sync logs", "error", err)
	}

	// Add failed files to map
	for _, fileID := range failedFiles {
		filesMap[fileID] = true
	}

	// Convert map to slice
	for fileID := range filesMap {
		unsyncedFiles = append(unsyncedFiles, fileID)
	}

	fileCount = len(unsyncedFiles)
	totalEntitiesToSync += fileCount

	return
}

// Update handles messages and updates the model
func (m SyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keymap.Enter):
			if !m.syncing && m.result != nil {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.ready = true
		}

		m.help.Width = msg.Width

	case spinner.TickMsg:
		if m.syncing {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case SyncStartMsg:
		m.syncing = true
		m.currentStage = "Preparing to sync..."
		return m, tea.Batch(
			m.spinner.Tick,
			m.startSync(),
		)

	case SyncProgressMsg:
		m.lastProgress = msg
		m.currentStage = fmt.Sprintf("Syncing %s: %d/%d", msg.EntityType, msg.CurrentItem, msg.TotalItems)
		m.progress.SetPercent(msg.ProgressValue)
		return m, nil

	case SyncCompleteMsg:
		m.syncing = false
		m.result = msg.Result
		m.workspaceCount = msg.WorkspaceCount
		m.reviewCount = msg.ReviewCount
		m.reviewFileCount = msg.ReviewFileCount
		m.issueCount = msg.IssueCount
		m.fileCount = msg.FileCount

		if msg.Error != nil {
			m.error = msg.Error.Error()
			m.status = "Sync failed"
		} else {
			m.status = "Sync complete"
		}

		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m SyncModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.styles.Title.Render("Mindnest Sync"))
	if m.dryRun {
		sb.WriteString(" " + m.styles.Info.Render("[DRY RUN]"))
	}
	sb.WriteString("\n\n")

	// Main content
	if m.syncing {
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " ", m.currentStage))
		sb.WriteString("\n\n")
		sb.WriteString(m.progress.View())
		sb.WriteString("\n\n")
	} else if m.result != nil {
		// Results
		if m.error != "" {
			sb.WriteString(m.styles.Error.Render("Error: " + m.error))
			sb.WriteString("\n\n")
		}

		resultBox := m.styles.Section.Copy().Width(m.width - 4)

		var resultContent strings.Builder
		resultContent.WriteString(fmt.Sprintf("Total items: %d\n", m.result.TotalItems))

		// Show a clear message when there's nothing to sync
		if m.result.TotalItems == 0 {
			resultContent.WriteString(m.styles.Info.Render("Nothing to sync! All items are up to date."))
			resultContent.WriteString("\n")
		} else {
			// Add breakdown by entity type if we have counts
			if m.workspaceCount > 0 || m.reviewCount > 0 || m.reviewFileCount > 0 || m.issueCount > 0 || m.fileCount > 0 {
				resultContent.WriteString("Breakdown:\n")
				if m.workspaceCount > 0 {
					resultContent.WriteString(fmt.Sprintf("  - Workspaces: %d\n", m.workspaceCount))
				}
				if m.reviewCount > 0 {
					resultContent.WriteString(fmt.Sprintf("  - Reviews: %d\n", m.reviewCount))
				}
				if m.reviewFileCount > 0 {
					resultContent.WriteString(fmt.Sprintf("  - Review Files: %d\n", m.reviewFileCount))
				}
				if m.issueCount > 0 {
					resultContent.WriteString(fmt.Sprintf("  - Issues: %d\n", m.issueCount))
				}
				if m.fileCount > 0 {
					resultContent.WriteString(fmt.Sprintf("  - Files: %d\n", m.fileCount))
				}
				resultContent.WriteString("\n")
			}

			resultContent.WriteString(fmt.Sprintf("Successfully synced: %d\n", m.result.SuccessItems))
			resultContent.WriteString(fmt.Sprintf("Failed: %d\n", m.result.FailedItems))
		}

		resultContent.WriteString(fmt.Sprintf("Duration: %s\n", m.result.Duration.Round(time.Millisecond)))

		if m.result.Success {
			resultContent.WriteString("\n" + m.styles.Success.Render("Sync completed successfully!"))
		} else {
			resultContent.WriteString("\n" + m.styles.Error.Render("Sync completed with errors"))
			if m.result.ErrorMessage != "" {
				resultContent.WriteString("\n" + m.styles.Error.Render("Error: "+m.result.ErrorMessage))
			}
		}

		sb.WriteString(resultBox.Render(resultContent.String()))
		sb.WriteString("\n\n")
		sb.WriteString(m.styles.Subtle.Render("Press Enter to exit"))
	} else {
		// Initial state
		if !m.app.Sync.IsConfigured() {
			sb.WriteString(m.styles.Info.Render("Sync is not configured. Use 'mindnest sync account link --token <token>' to configure"))
		} else {
			sb.WriteString(m.styles.Info.Render("Ready to sync..."))
		}
	}

	// Help
	if m.showHelp {
		sb.WriteString("\n\n" + m.help.View(m.keymap))
	} else {
		sb.WriteString("\n\n" + m.help.ShortHelpView(m.keymap.ShortHelp()))
	}

	return m.styles.Border.Render(sb.String())
}
